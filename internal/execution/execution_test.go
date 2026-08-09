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
		ApproveBy: time.Now().Add(time.Minute), EntryTTL: time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func approved(plan domain.TradePlan) domain.Approval {
	return domain.Approval{PlanID: plan.ID(), PlanVersion: plan.Version(), Decision: domain.ApprovalApproved}
}

func TestControllerRejectsUnapprovedPlan(t *testing.T) {
	ctx := context.Background()
	engine := &execution.SimulationEngine{}
	controller := execution.NewExecutionController(engine)
	plan := fakePlan(t)
	tests := []domain.Approval{
		{},
		{PlanID: plan.ID(), PlanVersion: plan.Version(), Decision: domain.ApprovalRejected},
		{PlanID: plan.ID(), PlanVersion: plan.Version() + 1, Decision: domain.ApprovalApproved},
	}
	for _, decision := range tests {
		if _, err := controller.Execute(ctx, plan, decision); !errors.Is(err, execution.ErrPlanNotApproved) {
			t.Fatalf("Execute approval %+v error = %v, want ErrPlanNotApproved", decision, err)
		}
	}
	if _, ok := engine.Current(); ok {
		t.Fatal("controller invoked engine for an unapproved plan")
	}
}

func TestControllerRejectsSecondActiveLifecycle(t *testing.T) {
	ctx := context.Background()
	controller := execution.NewExecutionController(&execution.SimulationEngine{})
	plan := fakePlan(t)
	if _, err := controller.Execute(ctx, plan, approved(plan)); err != nil {
		t.Fatal(err)
	}
	if _, err := controller.Execute(ctx, plan, approved(plan)); !errors.Is(err, execution.ErrActiveLifecycle) {
		t.Fatalf("second execution error = %v, want ErrActiveLifecycle", err)
	}
}

func TestEngineExecutesAuthorizedPlan(t *testing.T) {
	engine := &execution.SimulationEngine{}
	plan := fakePlan(t)
	if _, err := engine.Close(context.Background()); !errors.Is(err, execution.ErrInvalidLifecycleTransition) {
		t.Fatalf("close before execution error = %v, want ErrInvalidLifecycleTransition", err)
	}
	state, err := engine.Execute(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	if state.PlanID != plan.ID() || !state.FilledQuantity.Equal(plan.Quantity()) ||
		!state.ProtectedQuantity.Equal(plan.Quantity()) || state.PositionStatus != domain.PositionOpen {
		t.Fatalf("unexpected execution state: %+v", state)
	}
	if _, err := engine.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Close(context.Background()); !errors.Is(err, execution.ErrInvalidLifecycleTransition) {
		t.Fatalf("repeated close error = %v, want ErrInvalidLifecycleTransition", err)
	}
}

func TestClosedLifecycleAllowsNextPlan(t *testing.T) {
	ctx := context.Background()
	controller := execution.NewExecutionController(&execution.SimulationEngine{})
	first := fakePlan(t)
	if _, err := controller.Execute(ctx, first, approved(first)); err != nil {
		t.Fatal(err)
	}
	if _, err := controller.Close(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := controller.Close(ctx); !errors.Is(err, execution.ErrInvalidLifecycleTransition) {
		t.Fatalf("second close error = %v, want ErrInvalidLifecycleTransition", err)
	}
	entry := domain.MustDecimal("10.5")
	second, err := first.Edit(domain.TradePlanEdits{EntryPrice: &entry})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := controller.Execute(ctx, second, approved(second)); err != nil {
		t.Fatalf("next execution after close: %v", err)
	}
}

func TestSimulationEngineRespectsCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := (&execution.SimulationEngine{}).Execute(ctx, fakePlan(t)); !errors.Is(err, context.Canceled) {
		t.Fatalf("Execute error = %v, want context.Canceled", err)
	}
}

func TestFakePipelineHappyPath(t *testing.T) {
	ctx := context.Background()
	now := time.Unix(1_700_000_000, 0).UTC()
	source := &market.FakeCandleSource{Candles: []domain.Candle{{
		Market: domain.Market{Base: "BTC", Quote: "USD", Instrument: "BTC_USD"},
		Open:   domain.MustDecimal("64000"), High: domain.MustDecimal("64200"), Low: domain.MustDecimal("63900"), Close: domain.MustDecimal("64100"),
		OpenTime: now.Add(-time.Minute), CloseTime: now,
	}}}
	candle, err := source.NextCandle(ctx)
	if err != nil {
		t.Fatal(err)
	}
	detected, err := (signal.FakeDetector{}).Detect(ctx, candle)
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
	controller := execution.NewExecutionController(&execution.SimulationEngine{})
	state, err := controller.Execute(ctx, plans[0], decision)
	if err != nil {
		t.Fatal(err)
	}
	if !state.QuantitiesValid() || state.HasProtectionRisk() || state.PositionStatus != domain.PositionOpen {
		t.Fatalf("unexpected execution state: %+v", state)
	}
}
