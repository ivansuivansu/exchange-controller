package planner_test

import (
	"context"
	"testing"
	"time"

	"github.com/ivansuivansu/exchange-controller/internal/domain"
	"github.com/ivansuivansu/exchange-controller/internal/planner"
)

func recoveryIdea(high, low, recovery string) domain.TradeIdea {
	return domain.TradeIdea{
		ID: "idea-1", SignalIDs: []string{"signal-1"},
		Market:         domain.Market{Base: "BTC", Quote: "USD", Instrument: "BTC_USD"},
		ReferencePrice: domain.MustDecimal(recovery), CreatedAt: time.Unix(1000, 0),
		Recovery: &domain.RecoverySignal{
			RecentHigh: domain.MustDecimal(high), LocalLow: domain.MustDecimal(low),
			RecoveryPrice: domain.MustDecimal(recovery), DrawdownPercent: domain.MustDecimal("0.1"),
			RecoveryPercent: domain.MustDecimal("0.05"),
		},
	}
}

func baseConfig() planner.V1Config {
	return planner.V1Config{
		Name: "planner-v1", EntryMode: planner.EntryRecoveryClose,
		TakeProfitMode: planner.TakeProfitPreviousHigh, RetracementTarget: domain.MustDecimal("0.5"),
		StopLossOffset: domain.MustDecimal("0.05"), MinimumRiskReward: domain.MustDecimal("0.5"),
		ApprovalTTL: 5 * time.Minute, EntryTTL: 20 * time.Minute,
		FixedQuoteReserve: domain.MustDecimal("100"),
	}
}

func capital(amount string) domain.CapitalSnapshot {
	return domain.CapitalSnapshot{QuoteAsset: "USD", AvailableQuote: domain.MustDecimal(amount), AsOf: time.Unix(900, 0)}
}

func planOne(t *testing.T, config planner.V1Config, idea domain.TradeIdea, capital domain.CapitalSnapshot) domain.TradePlan {
	t.Helper()
	plans, err := (planner.PlannerV1{Config: config}).Plan(context.Background(), idea, capital)
	if err != nil {
		t.Fatal(err)
	}
	if len(plans) != 1 {
		t.Fatalf("plan count = %d", len(plans))
	}
	return plans[0]
}

func TestRecoveryClosePreviousHighAndStopBelowLow(t *testing.T) {
	plan := planOne(t, baseConfig(), recoveryIdea("110", "90", "100"), capital("1000"))
	if plan.EntryPrice().String() != "100" {
		t.Fatalf("entry = %s", plan.EntryPrice())
	}
	if plan.TakeProfit().String() != "110" {
		t.Fatalf("TP = %s", plan.TakeProfit())
	}
	if plan.StopLoss().String() != "85.5" || !plan.StopLoss().Less(domain.MustDecimal("90")) {
		t.Fatalf("SL = %s", plan.StopLoss())
	}
	if plan.QuoteNotional().String() != "900" || plan.Quantity().String() != "9" {
		t.Fatalf("capital plan: notional=%s qty=%s", plan.QuoteNotional(), plan.Quantity())
	}
	if plan.GrossUpsidePercent().String() != "0.1" || plan.DownsidePercent().String() != "0.145" || plan.RiskReward().String() != "0.68965517" {
		t.Fatalf("projections: upside=%s downside=%s RR=%s", plan.GrossUpsidePercent(), plan.DownsidePercent(), plan.RiskReward())
	}
}

func TestOffsetEntry(t *testing.T) {
	config := baseConfig()
	config.EntryMode = planner.EntryOffsetFromRecovery
	config.EntryOffset = domain.MustDecimal("-0.02")
	plan := planOne(t, config, recoveryIdea("110", "90", "100"), capital("1000"))
	if plan.EntryPrice().String() != "98" {
		t.Fatalf("offset entry = %s", plan.EntryPrice())
	}
}

func TestRetracementTakeProfit(t *testing.T) {
	config := baseConfig()
	config.TakeProfitMode = planner.TakeProfitRetracement
	config.RetracementTarget = domain.MustDecimal("0.25")
	config.MinimumRiskReward = domain.MustDecimal("0.1")
	plan := planOne(t, config, recoveryIdea("110", "90", "94"), capital("1000"))
	if plan.TakeProfit().String() != "95" {
		t.Fatalf("retracement TP = %s", plan.TakeProfit())
	}
}

func TestPlannerRejections(t *testing.T) {
	tests := []struct {
		name    string
		config  planner.V1Config
		idea    domain.TradeIdea
		capital domain.CapitalSnapshot
		reason  planner.RejectionReason
	}{
		{"invalid TP", baseConfig(), recoveryIdea("110", "90", "111"), capital("1000"), planner.RejectInvalidTakeProfit},
		{"invalid entry", func() planner.V1Config {
			c := baseConfig()
			c.EntryMode = planner.EntryOffsetFromRecovery
			c.EntryOffset = domain.MustDecimal("-1")
			return c
		}(), recoveryIdea("110", "90", "100"), capital("1000"), planner.RejectInvalidEntry},
		{"insufficient capital", func() planner.V1Config { c := baseConfig(); c.FixedQuoteReserve = domain.MustDecimal("1000"); return c }(), recoveryIdea("110", "90", "100"), capital("1000"), planner.RejectInsufficientCapital},
		{"minimum RR", func() planner.V1Config { c := baseConfig(); c.MinimumRiskReward = domain.MustDecimal("2"); return c }(), recoveryIdea("101", "90", "100"), capital("1000"), planner.RejectMinimumRiskReward},
		{"invalid SL", func() planner.V1Config {
			c := baseConfig()
			c.EntryMode = planner.EntryOffsetFromRecovery
			c.EntryOffset = domain.MustDecimal("-0.2")
			return c
		}(), recoveryIdea("110", "90", "100"), capital("1000"), planner.RejectInvalidStopLoss},
		{"malformed signal", baseConfig(), domain.TradeIdea{ID: "bad"}, capital("1000"), planner.RejectMalformedSignal},
		{"quantity rounds to zero", func() planner.V1Config { c := baseConfig(); c.FixedQuoteReserve = domain.Decimal{}; return c }(), recoveryIdea("100000000", "80000000", "90000000"), capital("0.00000001"), planner.RejectInvalidQuantity},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := (planner.PlannerV1{Config: tt.config}).Plan(context.Background(), tt.idea, tt.capital); !planner.IsRejection(err, tt.reason) {
				t.Fatalf("error = %v, want reason %s", err, tt.reason)
			}
		})
	}
}

func TestQuantityRoundsDownAndCostNeverExceedsCapital(t *testing.T) {
	config := baseConfig()
	config.FixedQuoteReserve = domain.Decimal{}
	config.MinimumRiskReward = domain.MustDecimal("0.1")
	plan := planOne(t, config, recoveryIdea("4", "2.5", "3"), capital("10"))
	if plan.Quantity().String() != "3.33333333" {
		t.Fatalf("quantity = %s", plan.Quantity())
	}
	if plan.QuoteNotional().String() != "9.99999999" {
		t.Fatalf("notional = %s", plan.QuoteNotional())
	}
	if domain.MustDecimal("10").Less(plan.QuoteNotional()) {
		t.Fatal("planned cost exceeds available capital")
	}
}

func TestTradePlanIntentRemainsImmutable(t *testing.T) {
	plan := planOne(t, baseConfig(), recoveryIdea("110", "90", "100"), capital("1000"))
	originalNotional, originalRR := plan.QuoteNotional(), plan.RiskReward()
	approveBy := plan.ApproveBy().Add(time.Minute)
	revised, err := plan.Edit(domain.TradePlanEdits{ApproveBy: &approveBy})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Version() != 1 || !plan.QuoteNotional().Equal(originalNotional) || !plan.RiskReward().Equal(originalRR) {
		t.Fatal("creating revision mutated original plan")
	}
	if revised.Version() != 2 {
		t.Fatalf("revision version = %d", revised.Version())
	}
}
