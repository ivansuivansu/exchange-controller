package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	ossignal "os/signal"
	"path/filepath"
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
	"github.com/ivansuivansu/exchange-controller/internal/notifier"
	"github.com/ivansuivansu/exchange-controller/internal/planner"
	signals "github.com/ivansuivansu/exchange-controller/internal/signal"
	"github.com/ivansuivansu/exchange-controller/internal/telegram"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "notify" {
		if err := runNotifier(); err != nil {
			log.Fatal(err)
		}
		return
	}
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

func runNotifier() error {
	settings, err := config.LoadNotifierFromEnv()
	if err != nil {
		return err
	}
	if !settings.Enabled {
		return fmt.Errorf("notifier is disabled; set NOTIFIER_ENABLED=true")
	}
	telegramConfig, err := config.LoadTelegramFromEnv()
	if err != nil {
		return err
	}
	ctx, stop := signalNotifyContext()
	defer stop()
	bot, err := telegram.NewBotAPI(telegramConfig.Token, nil)
	if err != nil {
		return err
	}
	monitor, err := notifier.New(notifier.Config{Market: cryptocom.BTCUSD, PollInterval: settings.PollInterval,
		WindowSize: settings.WindowSize, DropThreshold: settings.DropThreshold, Cooldown: settings.Cooldown})
	if err != nil {
		return err
	}
	source, err := cryptocom.NewSource(cryptocom.Config{Market: cryptocom.BTCUSD, PollInterval: settings.PollInterval,
		MaxAttempts: 3, RetryBackoff: time.Second})
	if err != nil {
		return err
	}
	handler := notifier.NewCommandHandler(monitor, bot, telegramConfig.AllowedUserIDs, telegramConfig.AllowedChatIDs)
	errors := make(chan error, 2)
	go func() { errors <- bot.Run(ctx, handler) }()
	go func() {
		errors <- (notifier.Service{Source: source, Monitor: monitor, Sender: bot, ChatID: telegramConfig.SendChatID}).Run(ctx)
	}()
	log.Printf("INFORMATION ONLY: BTC_USD decline notifier enabled; no trade can be created or executed")
	select {
	case err := <-errors:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func runBacktest() error {
	flags := flag.NewFlagSet("backtest", flag.ContinueOnError)
	filePath := flags.String("file", "", "read historical candle CSV")
	fromText := flags.String("from", "", "range start (RFC3339 or YYYY-MM-DD)")
	toText := flags.String("to", "", "range end, exclusive (RFC3339 or YYYY-MM-DD)")
	cachePath := flags.String("cache", "", "save downloaded candle CSV")
	tradePath := flags.String("trades", "backtest_trades.csv", "trade CSV output (empty disables)")
	monthlyPath := flags.String("monthly", "backtest_monthly.csv", "monthly CSV output (empty disables)")
	reportPath := flags.String("report", "", "optional text report output")
	feeText := flags.String("fee-rate", os.Getenv("BACKTEST_FEE_RATE"), "fractional simulation fee rate; e.g. 0.001")
	slippageText := flags.String("slippage-rate", os.Getenv("BACKTEST_SLIPPAGE_RATE"), "fractional adverse exit slippage")
	if err := flags.Parse(os.Args[2:]); err != nil {
		return err
	}
	if *feeText == "" {
		*feeText = "0"
	}
	if *slippageText == "" {
		*slippageText = "0"
	}
	fee, err := domain.ParseDecimal(*feeText)
	if err != nil {
		return fmt.Errorf("fee rate: %w", err)
	}
	slippage, err := domain.ParseDecimal(*slippageText)
	if err != nil {
		return fmt.Errorf("slippage rate: %w", err)
	}
	research := backtest.PlaceholderBTCUSDResearchPreset(fee, slippage)
	timeframe := research.Simulation.Timeframe
	from, err := parseOptionalTime(*fromText)
	if err != nil {
		return fmt.Errorf("from: %w", err)
	}
	to, err := parseOptionalTime(*toText)
	if err != nil {
		return fmt.Errorf("to: %w", err)
	}
	if (!from.IsZero() || !to.IsZero()) && (from.IsZero() || to.IsZero() || !from.Before(to)) {
		return fmt.Errorf("both --from and --to are required and from must precede to")
	}
	var candles []domain.Candle
	if *filePath != "" {
		candles, err = backtest.LoadCandleCSV(*filePath, research.Simulation.Market, timeframe, from, to)
	} else {
		if from.IsZero() || to.IsZero() {
			return fmt.Errorf("use --file or provide both --from and --to")
		}
		ctx, stop := signalNotifyContext()
		defer stop()
		source, sourceErr := cryptocom.NewSource(cryptocom.Config{HTTPClient: &http.Client{Timeout: 10 * time.Second}, Market: research.Simulation.Market, MaxAttempts: 3, RetryBackoff: time.Second, CandleTimeframe: research.Timeframe, CandleCount: 300})
		if sourceErr != nil {
			return sourceErr
		}
		candles, err = source.LoadCompletedCandlesRange(ctx, from, to)
		if err == nil && *cachePath != "" {
			err = writeFile(*cachePath, func(w io.Writer) error { return backtest.WriteCandleCSV(w, candles) })
		}
	}
	if err != nil {
		return err
	}
	if len(candles) == 0 {
		return fmt.Errorf("dataset contains no completed candles in requested range")
	}
	ctx, stop := signalNotifyContext()
	defer stop()
	history, err := backtest.NewInMemoryCandleSource(candles, research.Simulation.Market, timeframe)
	if err != nil {
		return err
	}
	detector, err := signals.NewDrawdownRecoveryDetector(signals.DrawdownRecoveryConfig{
		WindowSize: research.Signal.WindowSize, DrawdownThreshold: research.Signal.DrawdownThreshold,
		RecoveryThreshold: research.Signal.RecoveryThreshold, Cooldown: research.Signal.Cooldown,
	})
	if err != nil {
		return err
	}
	executionEngine, err := backtest.NewBacktestExecutionEngine(research.Simulation)
	if err != nil {
		return err
	}
	report, err := (backtest.Backtester{
		Source: history, Detector: detector, Ideas: idea.RecoveryBuilder{},
		Planner: planner.PlannerV1{Config: research.Planner}, Execution: executionEngine,
	}).Run(ctx)
	if err != nil {
		return err
	}
	identity := backtest.DatasetIdentity{FirstCandle: candles[0].OpenTime, LastCandle: candles[len(candles)-1].OpenTime, CandleCount: len(candles), Timeframe: research.Timeframe, Instrument: research.Simulation.Market.Instrument}
	if err := backtest.WriteResearchReport(os.Stdout, research, identity, report); err != nil {
		return err
	}
	if *reportPath != "" {
		if err := writeFile(*reportPath, func(w io.Writer) error { return backtest.WriteResearchReport(w, research, identity, report) }); err != nil {
			return err
		}
	}
	if *tradePath != "" {
		if err := writeFile(*tradePath, func(w io.Writer) error { return backtest.WriteTradeCSV(w, report.Trades) }); err != nil {
			return err
		}
	}
	if *monthlyPath != "" {
		if err := writeFile(*monthlyPath, func(w io.Writer) error { return backtest.WriteMonthlyCSV(w, backtest.MonthlyBreakdown(report)) }); err != nil {
			return err
		}
	}
	return nil
}

func parseOptionalTime(value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, nil
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed.UTC(), nil
	}
	parsed, err := time.Parse("2006-01-02", value)
	return parsed.UTC(), err
}

func writeFile(path string, write func(io.Writer) error) error {
	dir := filepath.Dir(path)
	if dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := write(file); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
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
