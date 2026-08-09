package execution

import (
	"context"
	"errors"
	"sync"

	"github.com/ivansuivansu/exchange-controller/internal/domain"
)

var ErrInvalidLifecycleTransition = errors.New("invalid execution lifecycle transition")

// ExecutionEngine executes a TradePlan that its caller has already authorized.
// Authorization and portfolio-level lifecycle policy do not belong to adapters.
type ExecutionEngine interface {
	Execute(context.Context, domain.TradePlan) (domain.ExecutionState, error)
	Close(context.Context) (domain.ExecutionState, error)
}

// SimulationEngine is a simple execution adapter: entry fills immediately and
// the complete fill is protected immediately.
type SimulationEngine struct {
	mu      sync.Mutex
	current *domain.ExecutionState
}

func (e *SimulationEngine) Execute(ctx context.Context, plan domain.TradePlan) (domain.ExecutionState, error) {
	if err := ctx.Err(); err != nil {
		return domain.ExecutionState{}, err
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return domain.ExecutionState{}, err
	}
	state := domain.ExecutionState{
		PlanID: plan.ID(), PlanVersion: plan.Version(),
		RequestedQuantity: plan.Quantity(), FilledQuantity: plan.Quantity(),
		ProtectedQuantity: plan.Quantity(), AverageEntryPrice: plan.EntryPrice(),
		EntryStatus: domain.EntryFilled, PositionStatus: domain.PositionOpen,
		Knowledge: domain.ExecutionKnown,
	}
	e.current = &state
	return state, nil
}

func (e *SimulationEngine) Current() (domain.ExecutionState, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.current == nil {
		return domain.ExecutionState{}, false
	}
	return *e.current, true
}

func (e *SimulationEngine) Close(ctx context.Context) (domain.ExecutionState, error) {
	if err := ctx.Err(); err != nil {
		return domain.ExecutionState{}, err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.current == nil || e.current.PositionStatus != domain.PositionOpen {
		return domain.ExecutionState{}, ErrInvalidLifecycleTransition
	}
	e.current.PositionStatus = domain.PositionClosed
	return *e.current, nil
}
