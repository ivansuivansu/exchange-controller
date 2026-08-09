package planner

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ivansuivansu/exchange-controller/internal/domain"
)

type SimpleConfig struct {
	Quantity    domain.Decimal
	TakeProfit  domain.Decimal
	StopLoss    domain.Decimal
	ApprovalTTL time.Duration
	EntryTTL    time.Duration
}

// SimplePlanner deliberately uses explicit configured TP/SL values. These are
// transparent MVP inputs, not an optimized strategy.
type SimplePlanner struct{ Config SimpleConfig }

var _ TradePlanner = SimplePlanner{}

func (p SimplePlanner) Plan(ctx context.Context, idea domain.TradeIdea) ([]domain.TradePlan, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !idea.ReferencePrice.IsPositive() || !p.Config.Quantity.IsPositive() ||
		!p.Config.TakeProfit.IsPositive() || !p.Config.StopLoss.IsPositive() ||
		p.Config.ApprovalTTL <= 0 || p.Config.EntryTTL <= 0 {
		return nil, errors.New("simple planner configuration and reference price must be positive")
	}
	plan, err := domain.NewTradePlan(domain.TradePlanParams{
		ID: fmt.Sprintf("plan-%d", idea.CreatedAt.UnixNano()), Version: 1, IdeaID: idea.ID,
		Market: idea.Market, EntryPrice: idea.ReferencePrice, Quantity: p.Config.Quantity,
		TakeProfit: p.Config.TakeProfit, StopLoss: p.Config.StopLoss,
		ApproveBy: idea.CreatedAt.Add(p.Config.ApprovalTTL), EntryTTL: p.Config.EntryTTL,
	})
	if err != nil {
		return nil, err
	}
	return []domain.TradePlan{plan}, nil
}
