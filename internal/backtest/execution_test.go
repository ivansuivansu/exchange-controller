package backtest

import (
	"errors"
	"testing"
	"time"

	"github.com/ivansuivansu/exchange-controller/internal/domain"
)

var btcusd = domain.Market{Base: "BTC", Quote: "USD", Instrument: "BTC_USD"}

func testPlan(t *testing.T, id string, quantity domain.Decimal, ttl time.Duration) domain.TradePlan {
	t.Helper()
	plan, err := domain.NewTradePlan(domain.TradePlanParams{
		ID: id, Version: 1, IdeaID: "idea-" + id, Market: btcusd,
		EntryPrice: domain.MustDecimal("100"), Quantity: quantity, QuoteNotional: domain.MustDecimal("100"),
		TakeProfit: domain.MustDecimal("110"), StopLoss: domain.MustDecimal("90"),
		ApproveBy: time.Unix(1000, 0), EntryTTL: ttl, RiskReward: domain.MustDecimal("1"),
		GrossUpsidePercent: domain.MustDecimal("0.1"), DownsidePercent: domain.MustDecimal("0.1"), PlannerName: "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	return plan
}

func testCandle(open time.Time, low, high, closePrice string) domain.Candle {
	return domain.Candle{Market: btcusd, Open: domain.MustDecimal(closePrice), High: domain.MustDecimal(high),
		Low: domain.MustDecimal(low), Close: domain.MustDecimal(closePrice), OpenTime: open, CloseTime: open.Add(time.Minute)}
}

func approved(plan domain.TradePlan, at time.Time) domain.Approval {
	return domain.Approval{PlanID: plan.ID(), PlanVersion: plan.Version(), Decision: domain.ApprovalApproved, DecidedAt: at}
}

func engine(t *testing.T, policy AmbiguityPolicy, fee, slippage string) *BacktestExecutionEngine {
	t.Helper()
	engine, err := NewBacktestExecutionEngine(SimulationConfig{Market: btcusd, Timeframe: time.Minute,
		StartingCapital: domain.MustDecimal("1000"), FeeRate: domain.MustDecimal(fee),
		SlippageRate: domain.MustDecimal(slippage), AmbiguityPolicy: policy})
	if err != nil {
		t.Fatal(err)
	}
	return engine
}

func TestEntryFillTPWinFeesSlippageAndNoSameCandleExit(t *testing.T) {
	start := time.Unix(2000, 0)
	plan := testPlan(t, "p1", domain.MustDecimal("1"), 5*time.Minute)
	e := engine(t, Conservative, "0.01", "0.01")
	if err := e.Submit(plan, approved(plan, start), start, start); err != nil {
		t.Fatal(err)
	}
	// This candle crosses entry, TP, and SL. It only fills entry; exits start next candle.
	result, err := e.OnCandle(testCandle(start, "80", "120", "100"))
	if err != nil || result != nil || !e.Active() {
		t.Fatalf("same-candle result=%+v err=%v", result, err)
	}
	result, err = e.OnCandle(testCandle(start.Add(time.Minute), "100", "111", "105"))
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitReason != ExitTakeProfit || result.EntryPriceActual.String() != "101" || result.ExitPrice.String() != "108.9" {
		t.Fatalf("unexpected fill/exit: %+v", result)
	}
	if result.EntryFee.String() != "1.01" || result.ExitFee.String() != "1.089" || result.NetPnL.String() != "5.801" {
		t.Fatalf("fees/PnL: %+v", result)
	}
}

func TestEntryExpiration(t *testing.T) {
	start := time.Unix(2000, 0)
	plan := testPlan(t, "expired", domain.MustDecimal("1"), time.Minute)
	e := engine(t, Conservative, "0", "0")
	if err := e.Submit(plan, approved(plan, start), start, start); err != nil {
		t.Fatal(err)
	}
	result, err := e.OnCandle(testCandle(start.Add(time.Minute), "99", "101", "100"))
	if err != nil || result == nil || result.ExitReason != ExitExpiration || !result.CapitalAfter.Equal(domain.MustDecimal("1000")) {
		t.Fatalf("expiration result=%+v err=%v", result, err)
	}
}

func TestSLLossAndAmbiguityPolicies(t *testing.T) {
	start := time.Unix(2000, 0)
	for _, tt := range []struct {
		policy AmbiguityPolicy
		want   ExitReason
	}{{Conservative, ExitStopLoss}, {Optimistic, ExitTakeProfit}} {
		t.Run(string(tt.policy), func(t *testing.T) {
			plan := testPlan(t, string(tt.policy), domain.MustDecimal("1"), 5*time.Minute)
			e := engine(t, tt.policy, "0", "0")
			if err := e.Submit(plan, approved(plan, start), start, start); err != nil {
				t.Fatal(err)
			}
			if _, err := e.OnCandle(testCandle(start, "99", "101", "100")); err != nil {
				t.Fatal(err)
			}
			result, err := e.OnCandle(testCandle(start.Add(time.Minute), "89", "111", "100"))
			if err != nil || result.ExitReason != tt.want || !result.WasAmbiguous {
				t.Fatalf("result=%+v err=%v", result, err)
			}
		})
	}
	// A non-ambiguous stop is always a loss.
	plan := testPlan(t, "sl", domain.MustDecimal("1"), 5*time.Minute)
	e := engine(t, Optimistic, "0", "0")
	_ = e.Submit(plan, approved(plan, start), start, start)
	_, _ = e.OnCandle(testCandle(start, "99", "101", "100"))
	result, _ := e.OnCandle(testCandle(start.Add(time.Minute), "89", "100", "95"))
	if result.ExitReason != ExitStopLoss || !result.NetPnL.Less(domain.Decimal{}) {
		t.Fatalf("SL result=%+v", result)
	}
}

func TestOneActiveLifecycleAndCapitalCompounding(t *testing.T) {
	start := time.Unix(2000, 0)
	e := engine(t, Conservative, "0", "0")
	first := testPlan(t, "first", domain.MustDecimal("1"), 5*time.Minute)
	second := testPlan(t, "second", domain.MustDecimal("1"), 5*time.Minute)
	if err := e.Submit(first, approved(first, start), start, start); err != nil {
		t.Fatal(err)
	}
	if err := e.Submit(second, approved(second, start), start, start); !errors.Is(err, ErrBacktestActiveLifecycle) {
		t.Fatalf("second submit error=%v", err)
	}
	_, _ = e.OnCandle(testCandle(start, "99", "101", "100"))
	result, _ := e.OnCandle(testCandle(start.Add(time.Minute), "100", "111", "105"))
	if result.CapitalAfter.String() != "1010" {
		t.Fatalf("first capital=%s", result.CapitalAfter)
	}
	if err := e.Submit(second, approved(second, start.Add(2*time.Minute)), start.Add(2*time.Minute), start.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	_, _ = e.OnCandle(testCandle(start.Add(2*time.Minute), "99", "101", "100"))
	result, _ = e.OnCandle(testCandle(start.Add(3*time.Minute), "100", "111", "105"))
	if result.CapitalBefore.String() != "1010" || result.CapitalAfter.String() != "1020" {
		t.Fatalf("compounded result=%+v", result)
	}
}
