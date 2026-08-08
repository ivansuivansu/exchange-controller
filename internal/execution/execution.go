package execution

import (
	"context"
	"errors"
	"sync"

	"github.com/ivansuivansu/exchange-controller/internal/domain"
)

var ErrActiveLifecycle = errors.New("an entry or position lifecycle is already active")

type ExecutionEngine interface {
	Execute(context.Context, domain.TradePlan, domain.Approval) (domain.ExecutionState, error)
}

// SimulationEngine fills and protects the requested quantity immediately. It
// retains the open lifecycle so a second plan cannot execute concurrently.
type SimulationEngine struct {
	mu      sync.Mutex
	current *domain.ExecutionState
}

func (e *SimulationEngine) Execute(_ context.Context, plan domain.TradePlan, approval domain.Approval) (domain.ExecutionState, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.current != nil && e.current.LifecycleActive() {
		return domain.ExecutionState{}, ErrActiveLifecycle
	}
	if approval.Decision != domain.ApprovalApproved || !approval.AppliesTo(plan) {
		return domain.ExecutionState{}, errors.New("plan does not have matching approval")
	}
	state := domain.ExecutionState{
		PlanID: plan.ID(), PlanVersion: plan.Version(),
		RequestedQuantity: plan.Quantity(), FilledQuantity: plan.Quantity(),
		ProtectedQuantity: plan.Quantity(), AverageEntryPrice: plan.EntryPrice(),
		EntryStatus: domain.EntryFilled, PositionStatus: domain.PositionOpen,
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

// Close completes the simulated lifecycle.
func (e *SimulationEngine) Close() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.current != nil {
		e.current.PositionStatus = domain.PositionClosed
	}
}
