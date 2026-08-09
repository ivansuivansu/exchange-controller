package idea

import (
	"context"

	"github.com/ivansuivansu/exchange-controller/internal/domain"
)

type IdeaBuilder interface {
	Build(context.Context, domain.Signal) (domain.TradeIdea, error)
}

type FakeBuilder struct{}

func (FakeBuilder) Build(_ context.Context, signal domain.Signal) (domain.TradeIdea, error) {
	return domain.TradeIdea{
		ID: "idea-1", SignalIDs: []string{signal.ID}, Market: signal.Market,
		CreatedAt: signal.ObservedAt, Description: "fake long idea",
	}, nil
}
