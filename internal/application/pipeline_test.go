package application_test

import (
	"context"
	"testing"
	"time"

	"github.com/ivansuivansu/exchange-controller/internal/domain"
	"github.com/ivansuivansu/exchange-controller/internal/execution"
	"github.com/ivansuivansu/exchange-controller/internal/idea"
	"github.com/ivansuivansu/exchange-controller/internal/planner"
	"github.com/ivansuivansu/exchange-controller/internal/signal"
	"github.com/ivansuivansu/exchange-controller/internal/telegram"
)

func TestRealSignalToTelegramApprovalToSimulatedExecution(t *testing.T) {
	ctx := context.Background()
	now := time.Unix(1_700_000_000, 0).UTC()
	market := domain.Market{Base: "BTC", Quote: "USD", Instrument: "BTC_USD"}
	detector, err := signal.NewDrawdownRecoveryDetector(signal.DrawdownRecoveryConfig{
		WindowSize: 5, DrawdownThreshold: domain.MustDecimal("0.10"),
		RecoveryThreshold: domain.MustDecimal("0.05"), Cooldown: time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	var detected domain.Signal
	for i, price := range []string{"100", "90", "94.5"} {
		openTime := now.Add(time.Duration(i) * time.Minute)
		detected, err = detector.Detect(ctx, domain.Candle{
			Market: market, Open: domain.MustDecimal(price), High: domain.MustDecimal(price),
			Low: domain.MustDecimal(price), Close: domain.MustDecimal(price),
			OpenTime: openTime, CloseTime: openTime.Add(time.Minute),
		})
	}
	if err != nil {
		t.Fatal(err)
	}
	tradeIdea, err := (idea.RecoveryBuilder{}).Build(ctx, detected)
	if err != nil {
		t.Fatal(err)
	}
	plans, err := (planner.SimplePlanner{Config: planner.SimpleConfig{
		Quantity: domain.MustDecimal("0.1"), TakeProfit: domain.MustDecimal("110"),
		StopLoss: domain.MustDecimal("80"), ApprovalTTL: time.Minute, EntryTTL: time.Minute,
	}}).Plan(ctx, tradeIdea)
	if err != nil {
		t.Fatal(err)
	}
	engine := &execution.SimulationEngine{}
	controller := execution.NewExecutionController(engine)
	approver := telegram.NewApprover(telegram.Config{
		SendChatID: 20, AllowedUserIDs: []int64{10}, AllowedChatIDs: []int64{20}, Mode: "SIMULATION",
	}, nil, controller)
	approver.SetClock(func() time.Time { return now.Add(3 * time.Second) })
	if err := approver.Present(ctx, plans[0]); err != nil {
		t.Fatal(err)
	}
	callbackResult, err := approver.HandleCallback(ctx, telegram.Callback{
		UserID: 10, ChatID: 20,
		Data: telegram.EncodeCallback(plans[0].ID(), plans[0].Version(), telegram.ActionApprove),
	})
	if err != nil {
		t.Fatal(err)
	}
	state, err := controller.Execute(ctx, plans[0], *callbackResult.Approval)
	if err != nil {
		t.Fatal(err)
	}
	if state.PositionStatus != domain.PositionOpen || state.HasProtectionRisk() || !state.FilledQuantity.Equal(plans[0].Quantity()) {
		t.Fatalf("unexpected simulated execution: %+v", state)
	}
}
