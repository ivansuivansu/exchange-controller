package signal

import (
	"context"

	"github.com/ivansuivansu/exchange-controller/internal/domain"
)

type SignalDetector interface {
	Detect(context.Context, domain.MarketEvent) (domain.Signal, error)
}

type FakeDetector struct{}

func (FakeDetector) Detect(_ context.Context, event domain.MarketEvent) (domain.Signal, error) {
	return domain.Signal{
		ID: "signal-1", Market: event.Market, ObservedAt: event.At,
		Price: event.Price, Description: "fake recovery signal",
	}, nil
}
