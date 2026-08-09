package backtest

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ivansuivansu/exchange-controller/internal/domain"
	"github.com/ivansuivansu/exchange-controller/internal/idea"
	"github.com/ivansuivansu/exchange-controller/internal/planner"
	"github.com/ivansuivansu/exchange-controller/internal/signal"
)

type Backtester struct {
	Source    HistoricalCandleSource
	Detector  signal.SignalDetector
	Ideas     idea.IdeaBuilder
	Planner   planner.TradePlanner
	Execution *BacktestExecutionEngine
}

type simulatedClockSetter interface{ SetClock(func() time.Time) }

func (b Backtester) Run(ctx context.Context) (Report, error) {
	if b.Source == nil || b.Detector == nil || b.Ideas == nil || b.Planner == nil || b.Execution == nil {
		return Report{}, errors.New("backtester dependencies are incomplete")
	}
	report := Report{StartingCapital: b.Execution.capital,
		PlannerRejectionsByReason: make(map[string]int)}
	for {
		candle, err := b.Source.NextCandle(ctx)
		if errors.Is(err, ErrHistoricalDataExhausted) {
			break
		}
		if err != nil {
			return Report{}, err
		}
		if b.Execution.Active() {
			result, err := b.Execution.OnCandle(candle)
			if err != nil {
				return Report{}, err
			}
			if result != nil {
				report.addResult(*result)
			}
			continue
		}
		if setter, ok := b.Detector.(simulatedClockSetter); ok {
			now := candle.CloseTime
			setter.SetClock(func() time.Time { return now })
		}
		detected, err := b.Detector.Detect(ctx, candle)
		if errors.Is(err, signal.ErrNoSignal) || errors.Is(err, signal.ErrDuplicateCandle) {
			continue
		}
		if err != nil {
			return Report{}, fmt.Errorf("backtest signal: %w", err)
		}
		report.TotalSignals++
		tradeIdea, err := b.Ideas.Build(ctx, detected)
		if err != nil {
			return Report{}, err
		}
		capital := domain.CapitalSnapshot{QuoteAsset: tradeIdea.Market.Quote,
			AvailableQuote: b.Execution.Capital(), AsOf: candle.CloseTime}
		plans, err := b.Planner.Plan(ctx, tradeIdea, capital)
		if err != nil {
			var rejection *planner.PlanRejectionError
			if errors.As(err, &rejection) {
				report.PlannerRejections++
				report.PlannerRejectionsByReason[string(rejection.Reason)]++
				continue
			}
			return Report{}, err
		}
		for _, plan := range plans {
			report.PlansProduced++
			approval := domain.Approval{PlanID: plan.ID(), PlanVersion: plan.Version(),
				Decision: domain.ApprovalApproved, DecidedAt: candle.CloseTime}
			if err := b.Execution.Submit(plan, approval, detected.ObservedAt, candle.CloseTime); err != nil {
				return Report{}, err
			}
			break // MVP: one lifecycle, even if a planner returns alternatives.
		}
	}
	report.finish(b.Execution.Capital())
	report.EntriesFilled = b.Execution.entriesFilled
	report.EntriesExpired = b.Execution.entriesExpired
	return report, nil
}
