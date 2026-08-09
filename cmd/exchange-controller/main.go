package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	ossignal "os/signal"
	"syscall"
	"time"

	"github.com/ivansuivansu/exchange-controller/internal/application"
	"github.com/ivansuivansu/exchange-controller/internal/approval"
	"github.com/ivansuivansu/exchange-controller/internal/config"
	"github.com/ivansuivansu/exchange-controller/internal/domain"
	"github.com/ivansuivansu/exchange-controller/internal/exchange/cryptocom"
	"github.com/ivansuivansu/exchange-controller/internal/execution"
	"github.com/ivansuivansu/exchange-controller/internal/idea"
	"github.com/ivansuivansu/exchange-controller/internal/market"
	"github.com/ivansuivansu/exchange-controller/internal/planner"
	signals "github.com/ivansuivansu/exchange-controller/internal/signal"
	"github.com/ivansuivansu/exchange-controller/internal/telegram"
)

func main() {
	if os.Getenv("APP_MODE") == "live-data-simulation" {
		if err := runLiveDataSimulation(); err != nil {
			log.Fatal(err)
		}
		return
	}
	runDemo()
}

func runLiveDataSimulation() error {
	settings, err := config.LoadLiveDataSimulationFromEnv()
	if err != nil {
		return err
	}
	telegramConfig, err := config.LoadTelegramFromEnv()
	if err != nil {
		return err
	}
	ctx, stop := signalNotifyContext()
	defer stop()
	client := &http.Client{Timeout: settings.HTTPTimeout}
	source, err := cryptocom.NewSource(cryptocom.Config{
		HTTPClient: client, Market: settings.Market, PollInterval: settings.PollInterval,
		MaxAttempts: settings.MaxAttempts, RetryBackoff: settings.RetryBackoff,
	})
	if err != nil {
		return err
	}
	detector, err := signals.NewDrawdownRecoveryDetector(signals.DrawdownRecoveryConfig{
		WindowSize: settings.WindowSize, DrawdownThreshold: settings.DrawdownThreshold,
		RecoveryThreshold: settings.RecoveryThreshold, Cooldown: settings.SignalCooldown,
	})
	if err != nil {
		return err
	}
	engine := &execution.SimulationEngine{}
	controller := execution.NewExecutionController(engine)
	bot, err := telegram.NewBotAPI(telegramConfig.Token, nil)
	if err != nil {
		return err
	}
	approver := telegram.NewApprover(telegram.Config{
		SendChatID: telegramConfig.SendChatID, AllowedUserIDs: telegramConfig.AllowedUserIDs,
		AllowedChatIDs: telegramConfig.AllowedChatIDs, Mode: "SIMULATION (LIVE MARKET DATA)",
	}, bot, controller)
	botErrors := make(chan error, 1)
	go func() { botErrors <- bot.Run(ctx, approver) }()
	log.Printf("SIMULATION ONLY: observing Crypto.com %s public data; no live orders can be submitted", settings.Market.Instrument)
	runner := application.Runner{
		Market: source, Signals: detector, Ideas: idea.RecoveryBuilder{},
		Planner: planner.SimplePlanner{Config: planner.SimpleConfig{
			Quantity: settings.Quantity, TakeProfit: settings.TakeProfit, StopLoss: settings.StopLoss,
			ApprovalTTL: settings.ApprovalTTL, EntryTTL: settings.EntryTTL,
		}}, Approver: approver, Execution: controller, Logger: log.Default(),
	}
	runnerErrors := make(chan error, 1)
	go func() { runnerErrors <- runner.Run(ctx) }()
	select {
	case err := <-botErrors:
		return err
	case err := <-runnerErrors:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func signalNotifyContext() (context.Context, context.CancelFunc) {
	return ossignal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
}

func runDemo() {
	ctx := context.Background()
	now := time.Now().UTC()
	source := &market.FakeSource{Event: domain.MarketEvent{
		Market: cryptocom.BTCUSD, Price: domain.MustDecimal("64100"), At: now,
	}}
	event, err := source.Next(ctx)
	if err != nil {
		log.Fatal(err)
	}
	detected, err := (signals.FakeDetector{}).Detect(ctx, event)
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
	fmt.Printf("SIMULATION %s plan %s v%d: filled %s, protected %s, position %s\n",
		plans[0].Market().Instrument, plans[0].ID(), plans[0].Version(),
		state.FilledQuantity, state.ProtectedQuantity, state.PositionStatus)
}
