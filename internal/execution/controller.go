package execution

import (
	"context"
	"errors"
	"sync"

	"github.com/ivansuivansu/exchange-controller/internal/domain"
)

var (
	ErrPlanNotApproved = errors.New("plan does not have matching approval")
	ErrActiveLifecycle = errors.New("an entry or position lifecycle is already active")
)

// ExecutionController is the application-level boundary that authorizes plans
// and enforces the single-active-lifecycle policy before invoking an adapter.
type ExecutionController struct {
	mu      sync.Mutex
	engine  ExecutionEngine
	current *domain.ExecutionState
}

func NewExecutionController(engine ExecutionEngine) *ExecutionController {
	return &ExecutionController{engine: engine}
}

func (c *ExecutionController) Execute(ctx context.Context, plan domain.TradePlan, approval domain.Approval) (domain.ExecutionState, error) {
	if err := ctx.Err(); err != nil {
		return domain.ExecutionState{}, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if approval.Decision != domain.ApprovalApproved || !approval.AppliesTo(plan) {
		return domain.ExecutionState{}, ErrPlanNotApproved
	}
	if c.current != nil && c.current.LifecycleActive() {
		return domain.ExecutionState{}, ErrActiveLifecycle
	}
	state, err := c.engine.Execute(ctx, plan)
	if err != nil {
		return domain.ExecutionState{}, err
	}
	c.current = &state
	return state, nil
}

func (c *ExecutionController) Close(ctx context.Context) (domain.ExecutionState, error) {
	if err := ctx.Err(); err != nil {
		return domain.ExecutionState{}, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.current == nil || c.current.PositionStatus != domain.PositionOpen {
		return domain.ExecutionState{}, ErrInvalidLifecycleTransition
	}
	state, err := c.engine.Close(ctx)
	if err != nil {
		return domain.ExecutionState{}, err
	}
	c.current = &state
	return state, nil
}

func (c *ExecutionController) Current() (domain.ExecutionState, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.current == nil {
		return domain.ExecutionState{}, false
	}
	return *c.current, true
}
