package market

import (
	"context"
	"errors"

	"github.com/ivansuivansu/exchange-controller/internal/domain"
)

type MarketDataSource interface {
	Next(context.Context) (domain.MarketEvent, error)
}

type CandleDataSource interface {
	NextCandle(context.Context) (domain.Candle, error)
}

type FakeSource struct {
	Event domain.MarketEvent
	used  bool
}

type FakeCandleSource struct {
	Candles []domain.Candle
	next    int
}

func (s *FakeCandleSource) NextCandle(context.Context) (domain.Candle, error) {
	if s.next >= len(s.Candles) {
		return domain.Candle{}, errors.New("fake candle source exhausted")
	}
	candle := s.Candles[s.next]
	s.next++
	return candle, nil
}

func (s *FakeSource) Next(context.Context) (domain.MarketEvent, error) {
	if s.used {
		return domain.MarketEvent{}, errors.New("fake market source exhausted")
	}
	s.used = true
	return s.Event, nil
}
