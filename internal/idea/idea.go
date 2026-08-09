package idea

import (
	"context"
	"fmt"

	"github.com/ivansuivansu/exchange-controller/internal/domain"
)

type IdeaBuilder interface {
	Build(context.Context, domain.Signal) (domain.TradeIdea, error)
}

type FakeBuilder struct{}

func (FakeBuilder) Build(_ context.Context, signal domain.Signal) (domain.TradeIdea, error) {
	return domain.TradeIdea{
		ID: "idea-1", SignalIDs: []string{signal.ID}, Market: signal.Market,
		ReferencePrice: signal.Price, CreatedAt: signal.ObservedAt, Description: "fake long idea",
	}, nil
}

type RecoveryBuilder struct{}

func (RecoveryBuilder) Build(_ context.Context, signal domain.Signal) (domain.TradeIdea, error) {
	return domain.TradeIdea{
		ID: fmt.Sprintf("idea-%d", signal.ObservedAt.UnixNano()), SignalIDs: []string{signal.ID},
		Market: signal.Market, ReferencePrice: signal.Price, CreatedAt: signal.ObservedAt,
		Description: "long idea from drawdown recovery signal", Recovery: cloneRecovery(signal.Recovery),
	}, nil
}

func cloneRecovery(source *domain.RecoverySignal) *domain.RecoverySignal {
	if source == nil {
		return nil
	}
	copy := *source
	return &copy
}
