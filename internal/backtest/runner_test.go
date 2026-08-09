package backtest

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/ivansuivansu/exchange-controller/internal/domain"
	"github.com/ivansuivansu/exchange-controller/internal/idea"
	"github.com/ivansuivansu/exchange-controller/internal/planner"
	"github.com/ivansuivansu/exchange-controller/internal/signal"
)

func historicalSequence(cycles int) []domain.Candle {
	start := time.Unix(10_000, 0)
	result := make([]domain.Candle, 0, cycles*5)
	for cycle := 0; cycle < cycles; cycle++ {
		base := start.Add(time.Duration(cycle*5) * time.Minute)
		result = append(result,
			testCandle(base, "99", "100", "99"),
			testCandle(base.Add(time.Minute), "90", "99", "91"),
			testCandle(base.Add(2*time.Minute), "91", "95", "94.5"),
			testCandle(base.Add(3*time.Minute), "94", "95", "94.5"),
			testCandle(base.Add(4*time.Minute), "96", "101", "100"),
		)
	}
	return result
}

func runProductionBacktest(t *testing.T, candles []domain.Candle, minimumRR string) Report {
	t.Helper()
	source, err := NewInMemoryCandleSource(candles, btcusd, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	detector, err := signal.NewDrawdownRecoveryDetector(signal.DrawdownRecoveryConfig{
		WindowSize: 5, DrawdownThreshold: domain.MustDecimal("0.10"),
		RecoveryThreshold: domain.MustDecimal("0.05"), Cooldown: 0,
	})
	if err != nil {
		t.Fatal(err)
	}
	executionEngine, err := NewBacktestExecutionEngine(SimulationConfig{
		Market: btcusd, Timeframe: time.Minute, StartingCapital: domain.MustDecimal("1000"),
		FeeRate: domain.Decimal{}, SlippageRate: domain.Decimal{}, AmbiguityPolicy: Conservative,
	})
	if err != nil {
		t.Fatal(err)
	}
	report, err := (Backtester{
		Source: source, Detector: detector, Ideas: idea.RecoveryBuilder{},
		Planner: planner.PlannerV1{Config: planner.V1Config{
			Name: "backtest-v1", EntryMode: planner.EntryRecoveryClose,
			TakeProfitMode: planner.TakeProfitPreviousHigh, RetracementTarget: domain.MustDecimal("0.5"),
			StopLossOffset: domain.MustDecimal("0.01"), MinimumRiskReward: domain.MustDecimal(minimumRR),
			ApprovalTTL: time.Minute, EntryTTL: 5 * time.Minute, FixedQuoteReserve: domain.Decimal{},
		}}, Execution: executionEngine,
	}).Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return report
}

func TestDeterministicReplayAndNoFutureLookahead(t *testing.T) {
	candles := historicalSequence(1)
	first := runProductionBacktest(t, candles, "0.5")
	second := runProductionBacktest(t, candles, "0.5")
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("replays differ:\n%+v\n%+v", first, second)
	}
	if len(first.Trades) != 1 || first.EntriesFilled != 1 || first.WinningTrades != 1 {
		t.Fatalf("report=%+v", first)
	}
	trade := first.Trades[0]
	if !trade.SignalTime.Before(trade.EntryFilledAt) || !trade.EntryFilledAt.Before(trade.ExitAt) {
		t.Fatalf("future timing violated: signal=%v fill=%v exit=%v", trade.SignalTime, trade.EntryFilledAt, trade.ExitAt)
	}
	if trade.EntrySubmittedAt != trade.SignalTime {
		t.Fatalf("approval/submission did not use simulated time")
	}
}

func TestCapitalCompoundsAcrossClosedTrades(t *testing.T) {
	report := runProductionBacktest(t, historicalSequence(2), "0.5")
	if len(report.Trades) != 2 {
		t.Fatalf("trades=%d report=%+v", len(report.Trades), report)
	}
	if !report.Trades[0].Quantity.Less(report.Trades[1].Quantity) {
		t.Fatalf("quantity did not compound: %s then %s", report.Trades[0].Quantity, report.Trades[1].Quantity)
	}
	if !report.StartingCapital.Less(report.EndingCapital) {
		t.Fatalf("capital did not grow: %s", report.EndingCapital)
	}
}

func TestPlannerRejectionIsCounted(t *testing.T) {
	report := runProductionBacktest(t, historicalSequence(1), "2")
	if report.TotalSignals != 1 || report.PlannerRejections != 1 || report.PlansProduced != 0 ||
		report.PlannerRejectionsByReason[string(planner.RejectMinimumRiskReward)] != 1 {
		t.Fatalf("rejection analytics=%+v", report)
	}
}

func TestConfiguredFeeLeavesEntryFeeCapital(t *testing.T) {
	candles := historicalSequence(1)
	source, err := NewInMemoryCandleSource(candles, btcusd, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	detector, _ := signal.NewDrawdownRecoveryDetector(signal.DrawdownRecoveryConfig{WindowSize: 5, DrawdownThreshold: domain.MustDecimal("0.10"), RecoveryThreshold: domain.MustDecimal("0.05")})
	engine, _ := NewBacktestExecutionEngine(SimulationConfig{Market: btcusd, Timeframe: time.Minute, StartingCapital: domain.MustDecimal("1000"), FeeRate: domain.MustDecimal("0.01"), AmbiguityPolicy: Conservative})
	report, err := (Backtester{Source: source, Detector: detector, Ideas: idea.RecoveryBuilder{}, Planner: planner.PlannerV1{Config: planner.V1Config{Name: "fee-test", EntryMode: planner.EntryRecoveryClose, TakeProfitMode: planner.TakeProfitPreviousHigh, StopLossOffset: domain.MustDecimal("0.01"), MinimumRiskReward: domain.MustDecimal("0.5"), ApprovalTTL: time.Minute, EntryTTL: 5 * time.Minute}}, Execution: engine}).Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if report.EntriesFilled != 1 || !report.TotalFeesPaid.IsPositive() {
		t.Fatalf("fee simulation report=%+v", report)
	}
}

func TestHistoricalSourceSortsAndRejectsMalformedData(t *testing.T) {
	candles := historicalSequence(1)[:2]
	source, err := NewInMemoryCandleSource([]domain.Candle{candles[1], candles[0]}, btcusd, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	first, _ := source.NextCandle(context.Background())
	if first.OpenTime != candles[0].OpenTime {
		t.Fatal("historical source did not sort chronologically")
	}
	if _, err := NewInMemoryCandleSource([]domain.Candle{candles[0], candles[0]}, btcusd, time.Minute); err == nil {
		t.Fatal("duplicate candles accepted")
	}
	bad := candles[0]
	bad.Low = domain.MustDecimal("101")
	if _, err := NewInMemoryCandleSource([]domain.Candle{bad}, btcusd, time.Minute); err == nil {
		t.Fatal("invalid OHLC accepted")
	}
}
