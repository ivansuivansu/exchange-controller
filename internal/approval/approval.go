package approval

import (
	"context"
	"time"

	"github.com/ivansuivansu/exchange-controller/internal/domain"
)

type PlanApprover interface {
	Decide(context.Context, domain.TradePlan) (domain.Approval, error)
}

type AutoApprover struct {
	Now func() time.Time
}

func (a AutoApprover) Decide(_ context.Context, plan domain.TradePlan) (domain.Approval, error) {
	now := time.Now()
	if a.Now != nil {
		now = a.Now()
	}
	return domain.Approval{
		PlanID: plan.ID(), PlanVersion: plan.Version(),
		Decision: domain.ApprovalApproved, DecidedAt: now,
	}, nil
}
