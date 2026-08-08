package execution_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ivansuivansu/exchange-controller/internal/approval"
	"github.com/ivansuivansu/exchange-controller/internal/domain"
	"github.com/ivansuivansu/exchange-controller/internal/execution"
	"github.com/ivansuivansu/exchange-controller/internal/idea"
	"github.com/ivansuivansu/exchange-controller/internal/market"
	"github.com/ivansuivansu/exchange-controller/internal/planner"
	"github.com/ivansuivansu/exchange-controller/internal/signal"
)

func fakePlan(t *testing.T) domain.TradePlan {
	t.Helper()
	p, err := domain.NewTradePlan(domain.TradePlanParams{
		ID: "p", Version: 1, IdeaID: "i",
		Market:     domain.Market{Base: "BTC", Quote: "USD", Instrument: "BTC_USD"},
		EntryPrice: domain.MustDecimal("10"), Quantity: domain.MustDecimal("1"),
		TakeProfit: domain.MustDecimal("11"), StopLoss: domain.MustDecimal("9"),
	})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestSingleActiveLifecycle(t *testing.T) {
	ctx := context.Background()
	engine := &execution.SimulationEngine{}
	plan := fakePlan(t)
	approved := domain.Approval{PlanID: plan.ID(), PlanVersion: plan.Version(), Decision: domain.ApprovalApproved}
	if _, err := engine.Execute(ctx, plan, approved); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Execute(ctx, plan, approved); !errors.Is(err, execution.ErrActiveLifecycle) {
		t.Fatalf("second execution error = %v, want ErrActiveLifecycle", err)
	}
	engine.Close()
	if _, err := engine.Execute(ctx, plan, approved); err != nil {
		t.Fatalf("execution after lifecycle close: %v", err)
	}
}

func TestFakePipelineHappyPath(t *testing.T) {
	ctx := context.Background()
	now := time.Unix(1_700_000_000, 0).UTC()
	source := &market.FakeSource{Event: domain.MarketEvent{
		Market: domain.Market{Base: "BTC", Quote: "USD", Instrument: "BTC_USD"},
		Price:  domain.MustDecimal("64100"), At: now,
	}}
	event, err := source.Next(ctx)
	if err != nil {
		t.Fatal(err)
	}
	detected, err := (signal.FakeDetector{}).Detect(ctx, event)
	if err != nil {
		t.Fatal(err)
	}
	tradeIdea, err := (idea.FakeBuilder{}).Build(ctx, detected)
	if err != nil {
		t.Fatal(err)
	}
	plans, err := (planner.FakePlanner{Now: func() time.Time { return now }}).Plan(ctx, tradeIdea)
	if err != nil {
		t.Fatal(err)
	}
	decision, err := (approval.AutoApprover{Now: func() time.Time { return now }}).Decide(ctx, plans[0])
	if err != nil {
		t.Fatal(err)
	}
	state, err := (&execution.SimulationEngine{}).Execute(ctx, plans[0], decision)
	if err != nil {
		t.Fatal(err)
	}
	if !state.QuantitiesValid() || state.HasProtectionRisk() || state.PositionStatus != domain.PositionOpen {
		t.Fatalf("unexpected execution state: %+v", state)
	}
}
