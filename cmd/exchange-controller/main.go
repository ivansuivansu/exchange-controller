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
	"github.com/ivansuivansu/exchange-controller/internal/backtest"
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
	if len(os.Args) > 1 && os.Args[1] == "backtest" {
		if err := runBacktest(); err != nil {
			log.Fatal(err)
		}
		return
	}
	if os.Getenv("APP_MODE") == "live-data-simulation" {
		if err := runLiveDataSimulation(); err != nil {
			log.Fatal(err)
		}
		return
	}
	runDemo()
}

func runBacktest() error {
	settings, err := config.LoadLiveDataSimulationFromEnv()
	if err != nil {
		return err
	}
	backtestSettings, err := config.LoadBacktestFromEnv()
	if err != nil {
		return err
	}
	timeframe, err := cryptocom.TimeframeDuration(settings.CandleTimeframe)
	if err != nil {
		return err
	}
	ctx, stop := signalNotifyContext()
	defer stop()
	source, err := cryptocom.NewSource(cryptocom.Config{
		HTTPClient: &http.Client{Timeout: settings.HTTPTimeout}, Market: settings.Market,
		MaxAttempts: settings.MaxAttempts, RetryBackoff: settings.RetryBackoff,
		CandleTimeframe: settings.CandleTimeframe, CandleCount: backtestSettings.CandleCount,
	})
	if err != nil {
		return err
	}
	candles, err := source.LoadCompletedCandles(ctx)
	if err != nil {
		return err
	}
	history, err := backtest.NewInMemoryCandleSource(candles, settings.Market, timeframe)
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
	executionEngine, err := backtest.NewBacktestExecutionEngine(backtest.SimulationConfig{
		Market: settings.Market, Timeframe: timeframe, StartingCapital: settings.Capital.AvailableQuote,
		FeeRate: backtestSettings.FeeRate, SlippageRate: backtestSettings.SlippageRate,
		AmbiguityPolicy: backtest.AmbiguityPolicy(backtestSettings.AmbiguityPolicy),
	})
	if err != nil {
		return err
	}
	report, err := (backtest.Backtester{
		Source: history, Detector: detector, Ideas: idea.RecoveryBuilder{},
		Planner: planner.PlannerV1{Config: settings.Planner}, Execution: executionEngine,
	}).Run(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("BACKTEST SIMULATION — %s %s\n", settings.Market.Instrument, settings.CandleTimeframe)
	fmt.Printf("Capital: %s → %s %s | Net: %s | Return: %s%%\n",
		report.StartingCapital, report.EndingCapital, settings.Market.Quote, report.NetProfitLoss, percent(report.TotalReturnPercent))
	fmt.Printf("Signals: %d | Plans: %d | Rejections: %d | Filled: %d | Expired: %d\n",
		report.TotalSignals, report.PlansProduced, report.PlannerRejections, report.EntriesFilled, report.EntriesExpired)
	fmt.Printf("Wins/Losses: %d/%d | Win rate: %s%% | Profit factor: %s | Max drawdown: %s%% | Fees: %s\n",
		report.WinningTrades, report.LosingTrades, percent(report.WinRate), report.ProfitFactor,
		percent(report.MaximumDrawdown), report.TotalFeesPaid)
	return nil
}

func percent(value domain.Decimal) string {
	result, err := value.Mul(domain.MustDecimal("100"), domain.RoundTowardZero)
	if err != nil {
		return value.String()
	}
	return result.String()
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
		CandleTimeframe: settings.CandleTimeframe, CandleCount: settings.WindowSize + 1,
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
		Planner: planner.PlannerV1{Config: settings.Planner}, Capital: settings.Capital,
		Approver: approver, Execution: controller, Logger: log.Default(),
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
	source := &market.FakeCandleSource{Candles: []domain.Candle{{
		Market: cryptocom.BTCUSD, Open: domain.MustDecimal("64000"), High: domain.MustDecimal("64200"),
		Low: domain.MustDecimal("63900"), Close: domain.MustDecimal("64100"),
		OpenTime: now.Add(-time.Minute), CloseTime: now,
	}}}
	candle, err := source.NextCandle(ctx)
	if err != nil {
		log.Fatal(err)
	}
	detected, err := (signals.FakeDetector{}).Detect(ctx, candle)
	if err != nil {
		log.Fatal(err)
	}
	tradeIdea, err := (idea.FakeBuilder{}).Build(ctx, detected)
	if err != nil {
		log.Fatal(err)
	}
	plans, err := (planner.FakePlanner{}).Plan(ctx, tradeIdea, domain.CapitalSnapshot{})
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
