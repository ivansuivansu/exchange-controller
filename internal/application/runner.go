package application

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/ivansuivansu/exchange-controller/internal/approval"
	"github.com/ivansuivansu/exchange-controller/internal/domain"
	"github.com/ivansuivansu/exchange-controller/internal/execution"
	"github.com/ivansuivansu/exchange-controller/internal/idea"
	"github.com/ivansuivansu/exchange-controller/internal/market"
	"github.com/ivansuivansu/exchange-controller/internal/planner"
	"github.com/ivansuivansu/exchange-controller/internal/signal"
)

type Runner struct {
	Market    market.CandleDataSource
	Signals   signal.SignalDetector
	Ideas     idea.IdeaBuilder
	Planner   planner.TradePlanner
	Approver  approval.PlanApprover
	Execution *execution.ExecutionController
	Logger    *log.Logger
}

func (r Runner) Run(ctx context.Context) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		candle, err := r.Market.NextCandle(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			r.logf("SIMULATION market-data error: %v", err)
			continue
		}
		if state, exists := r.Execution.Current(); exists && state.LifecycleActive() {
			continue
		}
		detected, err := r.Signals.Detect(ctx, candle)
		if errors.Is(err, signal.ErrNoSignal) || errors.Is(err, signal.ErrDuplicateCandle) || errors.Is(err, signal.ErrIncompleteCandle) {
			continue
		}
		if err != nil {
			return fmt.Errorf("detect signal: %w", err)
		}
		tradeIdea, err := r.Ideas.Build(ctx, detected)
		if err != nil {
			return fmt.Errorf("build idea: %w", err)
		}
		plans, err := r.Planner.Plan(ctx, tradeIdea)
		if err != nil {
			return fmt.Errorf("plan trade: %w", err)
		}
		for _, plan := range plans {
			r.logf("SIMULATION presenting %s v%d; no live order will be submitted", plan.ID(), plan.Version())
			decision, err := r.Approver.Decide(ctx, plan)
			if err != nil {
				return fmt.Errorf("approve plan: %w", err)
			}
			if decision.Decision == domain.ApprovalRejected {
				continue
			}
			state, err := r.Execution.Execute(ctx, plan, decision)
			if err != nil {
				return fmt.Errorf("simulate execution: %w", err)
			}
			r.logf("SIMULATION filled %s and protected %s", state.FilledQuantity, state.ProtectedQuantity)
		}
	}
}

func (r Runner) logf(format string, args ...any) {
	if r.Logger != nil {
		r.Logger.Printf(format, args...)
	}
}
