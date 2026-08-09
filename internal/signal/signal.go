package signal

import (
	"context"

	"github.com/ivansuivansu/exchange-controller/internal/domain"
)

type SignalDetector interface {
	Detect(context.Context, domain.Candle) (domain.Signal, error)
}

type FakeDetector struct{}

func (FakeDetector) Detect(_ context.Context, candle domain.Candle) (domain.Signal, error) {
	return domain.Signal{
		ID: "signal-1", Market: candle.Market, ObservedAt: candle.CloseTime,
		Price: candle.Close, Description: "fake recovery signal",
	}, nil
}
