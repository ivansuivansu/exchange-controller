package backtest

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/ivansuivansu/exchange-controller/internal/domain"
)

type HistoricalCandleSource interface {
	NextCandle(context.Context) (domain.Candle, error)
}

var ErrHistoricalDataExhausted = errors.New("historical candle data exhausted")

type InMemoryCandleSource struct {
	candles []domain.Candle
	next    int
}

func NewInMemoryCandleSource(input []domain.Candle, expected domain.Market, timeframe time.Duration) (*InMemoryCandleSource, error) {
	if timeframe <= 0 {
		return nil, errors.New("historical timeframe must be positive")
	}
	candles := append([]domain.Candle(nil), input...)
	sort.Slice(candles, func(i, j int) bool { return candles[i].OpenTime.Before(candles[j].OpenTime) })
	for i, candle := range candles {
		if candle.Market != expected {
			return nil, errors.New("historical candle market mismatch")
		}
		if candle.OpenTime.IsZero() || candle.CloseTime.Sub(candle.OpenTime) != timeframe {
			return nil, errors.New("historical candle timeframe mismatch")
		}
		if !validOHLC(candle) {
			return nil, errors.New("invalid historical OHLC candle")
		}
		if i > 0 && !candles[i-1].OpenTime.Before(candle.OpenTime) {
			return nil, errors.New("duplicate historical candle")
		}
	}
	return &InMemoryCandleSource{candles: candles}, nil
}

func validOHLC(c domain.Candle) bool {
	return c.Open.IsPositive() && c.High.IsPositive() && c.Low.IsPositive() && c.Close.IsPositive() &&
		!c.High.Less(c.Low) && !c.Open.Less(c.Low) && !c.High.Less(c.Open) &&
		!c.Close.Less(c.Low) && !c.High.Less(c.Close)
}

func (s *InMemoryCandleSource) NextCandle(ctx context.Context) (domain.Candle, error) {
	if err := ctx.Err(); err != nil {
		return domain.Candle{}, err
	}
	if s.next >= len(s.candles) {
		return domain.Candle{}, ErrHistoricalDataExhausted
	}
	candle := s.candles[s.next]
	s.next++
	return candle, nil
}
