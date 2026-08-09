package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/ivansuivansu/exchange-controller/internal/approval"
	"github.com/ivansuivansu/exchange-controller/internal/domain"
	"github.com/ivansuivansu/exchange-controller/internal/execution"
	"github.com/ivansuivansu/exchange-controller/internal/idea"
	"github.com/ivansuivansu/exchange-controller/internal/market"
	"github.com/ivansuivansu/exchange-controller/internal/planner"
	"github.com/ivansuivansu/exchange-controller/internal/signal"
)

func main() {
	ctx := context.Background()
	now := time.Now().UTC()
	source := &market.FakeSource{Event: domain.MarketEvent{
		Market: domain.Market{Base: "BTC", Quote: "USD", Instrument: "BTC_USD"},
		Price:  domain.MustDecimal("64100"), At: now,
	}}

	event, err := source.Next(ctx)
	if err != nil {
		log.Fatal(err)
	}
	detected, err := (signal.FakeDetector{}).Detect(ctx, event)
	if err != nil {
		log.Fatal(err)
	}
	tradeIdea, err := (idea.FakeBuilder{}).Build(ctx, detected)
	if err != nil {
		log.Fatal(err)
	}
	plans, err := (planner.FakePlanner{}).Plan(ctx, tradeIdea)
	if err != nil {
		log.Fatal(err)
	}
	decision, err := (approval.AutoApprover{}).Decide(ctx, plans[0])
	if err != nil {
		log.Fatal(err)
	}
	controller := execution.NewExecutionController(&execution.SimulationEngine{})
	state, err := controller.Execute(ctx, plans[0], decision)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("%s plan %s v%d: filled %s, protected %s, position %s\n",
		plans[0].Market().Instrument, plans[0].ID(), plans[0].Version(),
		state.FilledQuantity, state.ProtectedQuantity, state.PositionStatus)
}
