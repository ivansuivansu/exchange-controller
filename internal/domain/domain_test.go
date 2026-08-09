package domain_test

import (
	"testing"
	"time"

	"github.com/ivansuivansu/exchange-controller/internal/domain"
)

func planParams() domain.TradePlanParams {
	return domain.TradePlanParams{
		ID: "plan-1", Version: 1, IdeaID: "idea-1",
		Market:     domain.Market{Base: "BTC", Quote: "USD", Instrument: "BTC_USD"},
		EntryPrice: domain.MustDecimal("64100"), Quantity: domain.MustDecimal("0.01"),
		TakeProfit: domain.MustDecimal("65800"), StopLoss: domain.MustDecimal("63500"),
		ApproveBy: time.Unix(1000, 0), EntryTTL: 15 * time.Minute,
		QuoteNotional: domain.MustDecimal("641"), RiskReward: domain.MustDecimal("2"),
		GrossUpsidePercent: domain.MustDecimal("0.02"), DownsidePercent: domain.MustDecimal("0.01"), PlannerName: "test",
	}
}

func TestTradePlanVersionSemantics(t *testing.T) {
	original, err := domain.NewTradePlan(planParams())
	if err != nil {
		t.Fatal(err)
	}
	approval := domain.Approval{PlanID: original.ID(), PlanVersion: original.Version(), Decision: domain.ApprovalApproved}

	newEntry := domain.MustDecimal("64000")
	revised, err := original.Edit(domain.TradePlanEdits{EntryPrice: &newEntry})
	if err != nil {
		t.Fatal(err)
	}
	if revised.ID() != original.ID() || revised.Version() != original.Version()+1 {
		t.Fatalf("revision identity = %s v%d", revised.ID(), revised.Version())
	}
	if !original.EntryPrice().Equal(domain.MustDecimal("64100")) {
		t.Fatal("creating a revision mutated the original")
	}
	if !revised.Quantity().Equal(original.Quantity()) || revised.EntryTTL() != original.EntryTTL() {
		t.Fatal("unspecified material fields changed in revision")
	}
	if approval.AppliesTo(revised) {
		t.Fatal("approval for an old version must not apply to its revision")
	}
	if _, err := original.Edit(domain.TradePlanEdits{}); err == nil {
		t.Fatal("empty edit unexpectedly created a version")
	}
}

func TestTradePlanUsesSubmissionRelativeEntryTTL(t *testing.T) {
	plan, err := domain.NewTradePlan(planParams())
	if err != nil {
		t.Fatal(err)
	}
	if plan.ApproveBy() != time.Unix(1000, 0) || plan.EntryTTL() != 15*time.Minute {
		t.Fatalf("expiration semantics lost: approve by %v, TTL %v", plan.ApproveBy(), plan.EntryTTL())
	}
	submittedAt := time.Unix(2000, 0)
	if got := plan.EntryExpiresAt(submittedAt); got != submittedAt.Add(15*time.Minute) {
		t.Fatalf("entry expiry = %v", got)
	}
}

func TestTradePlanRequiresCompleteMarket(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*domain.TradePlanParams)
	}{
		{"base", func(p *domain.TradePlanParams) { p.Market.Base = "" }},
		{"quote", func(p *domain.TradePlanParams) { p.Market.Quote = "" }},
		{"instrument", func(p *domain.TradePlanParams) { p.Market.Instrument = "" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := planParams()
			tt.mutate(&params)
			if _, err := domain.NewTradePlan(params); err == nil {
				t.Fatal("NewTradePlan accepted incomplete market")
			}
		})
	}
}

func TestPlanLifecycleIsSeparateAndVersionSpecific(t *testing.T) {
	plan, err := domain.NewTradePlan(planParams())
	if err != nil {
		t.Fatal(err)
	}
	lifecycle := domain.TradePlanLifecycle{
		PlanID: plan.ID(), PlanVersion: plan.Version(), Status: domain.PlanApproved,
	}
	if lifecycle.PlanID != plan.ID() || lifecycle.Status != domain.PlanApproved {
		t.Fatal("plan lifecycle does not identify approved plan version")
	}
}

func TestTradeIdeaSupportsMultipleSignals(t *testing.T) {
	idea := domain.TradeIdea{SignalIDs: []string{"signal-1", "signal-2"}}
	if len(idea.SignalIDs) != 2 {
		t.Fatalf("signal count = %d, want 2", len(idea.SignalIDs))
	}
}

func TestExecutionQuantityInvariants(t *testing.T) {
	tests := []struct {
		name  string
		state domain.ExecutionState
		valid bool
	}{
		{"equal fill and protection", domain.ExecutionState{RequestedQuantity: domain.MustDecimal("1"), FilledQuantity: domain.MustDecimal("0.4"), ProtectedQuantity: domain.MustDecimal("0.4")}, true},
		{"protection exceeds fill", domain.ExecutionState{RequestedQuantity: domain.MustDecimal("1"), FilledQuantity: domain.MustDecimal("0.4"), ProtectedQuantity: domain.MustDecimal("0.5")}, false},
		{"fill exceeds request", domain.ExecutionState{RequestedQuantity: domain.MustDecimal("1"), FilledQuantity: domain.MustDecimal("1.1"), ProtectedQuantity: domain.MustDecimal("1.1")}, false},
		{"negative requested", domain.ExecutionState{RequestedQuantity: domain.MustDecimal("-1")}, false},
		{"negative filled", domain.ExecutionState{RequestedQuantity: domain.MustDecimal("1"), FilledQuantity: domain.MustDecimal("-0.1")}, false},
		{"negative protected", domain.ExecutionState{RequestedQuantity: domain.MustDecimal("1"), FilledQuantity: domain.MustDecimal("0.1"), ProtectedQuantity: domain.MustDecimal("-0.1")}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.state.QuantitiesValid(); got != tt.valid {
				t.Fatalf("QuantitiesValid() = %v, want %v", got, tt.valid)
			}
		})
	}
}

func TestExecutionStatesRepresentTerminalAndUncertainOutcomes(t *testing.T) {
	for _, status := range []domain.EntryExecutionState{domain.EntryExpired, domain.EntryRejected} {
		state := domain.ExecutionState{EntryStatus: status, Knowledge: domain.ExecutionKnown}
		if state.LifecycleActive() {
			t.Fatalf("terminal entry state %q is active", status)
		}
	}
	for _, knowledge := range []domain.ExecutionKnowledgeState{domain.ExecutionUnknown, domain.ExecutionAmbiguous} {
		state := domain.ExecutionState{Knowledge: knowledge}
		if !state.LifecycleActive() {
			t.Fatalf("uncertain execution state %q must block a new lifecycle", knowledge)
		}
	}
	if !(domain.ExecutionState{}).LifecycleActive() {
		t.Fatal("unset execution knowledge must fail safe as an active/uncertain lifecycle")
	}
}

func TestPartialFillProtectionRisk(t *testing.T) {
	state := domain.ExecutionState{
		RequestedQuantity: domain.MustDecimal("0.01"),
		FilledQuantity:    domain.MustDecimal("0.007"),
		ProtectedQuantity: domain.MustDecimal("0.004"),
	}
	if !state.QuantitiesValid() {
		t.Fatal("an under-protected state is risky but structurally valid")
	}
	if !state.HasProtectionRisk() {
		t.Fatal("expected partial-fill protection risk")
	}
	state.ProtectedQuantity = state.FilledQuantity
	if state.HasProtectionRisk() {
		t.Fatal("equal filled and protected quantities must not report risk")
	}
}
