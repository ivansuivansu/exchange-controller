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
		ApproveBy: time.Unix(1000, 0), EntryExpiresAt: time.Unix(2000, 0),
	}
}

func TestTradePlanVersionSemantics(t *testing.T) {
	original, err := domain.NewTradePlan(planParams())
	if err != nil {
		t.Fatal(err)
	}
	approval := domain.Approval{PlanID: original.ID(), PlanVersion: original.Version(), Decision: domain.ApprovalApproved}

	changed := planParams()
	changed.EntryPrice = domain.MustDecimal("64000")
	revised, err := original.NewVersion(changed)
	if err != nil {
		t.Fatal(err)
	}
	if revised.ID() != original.ID() || revised.Version() != original.Version()+1 {
		t.Fatalf("revision identity = %s v%d", revised.ID(), revised.Version())
	}
	if !original.EntryPrice().Equal(domain.MustDecimal("64100")) {
		t.Fatal("creating a revision mutated the original")
	}
	if approval.AppliesTo(revised) {
		t.Fatal("approval for an old version must not apply to its revision")
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.state.QuantitiesValid(); got != tt.valid {
				t.Fatalf("QuantitiesValid() = %v, want %v", got, tt.valid)
			}
		})
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
