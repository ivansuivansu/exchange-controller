package planner

import (
	"context"
	"time"

	"github.com/ivansuivansu/exchange-controller/internal/domain"
)

type TradePlanner interface {
	Plan(context.Context, domain.TradeIdea, domain.CapitalSnapshot) ([]domain.TradePlan, error)
}

type FakePlanner struct {
	Now func() time.Time
}

func (p FakePlanner) Plan(_ context.Context, idea domain.TradeIdea, _ domain.CapitalSnapshot) ([]domain.TradePlan, error) {
	now := idea.CreatedAt
	if p.Now != nil {
		now = p.Now()
	}
	plan, err := domain.NewTradePlan(domain.TradePlanParams{
		ID: "plan-1", Version: 1, IdeaID: idea.ID, Market: idea.Market,
		EntryPrice: domain.MustDecimal("64100"), Quantity: domain.MustDecimal("0.01"),
		TakeProfit: domain.MustDecimal("65800"), StopLoss: domain.MustDecimal("63500"),
		ApproveBy: now.Add(5 * time.Minute), EntryTTL: 25 * time.Minute,
		QuoteNotional: domain.MustDecimal("641"), RiskReward: domain.MustDecimal("2"),
		GrossUpsidePercent: domain.MustDecimal("0.0265"), DownsidePercent: domain.MustDecimal("0.0093"),
		PlannerName: "fake-planner",
	})
	if err != nil {
		return nil, err
	}
	return []domain.TradePlan{plan}, nil
}
