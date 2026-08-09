package market

import (
	"errors"
	"sync"

	"github.com/ivansuivansu/exchange-controller/internal/domain"
)

type RollingWindow struct {
	mu      sync.RWMutex
	size    int
	candles []domain.Candle
}

func NewRollingWindow(size int) (*RollingWindow, error) {
	if size <= 0 {
		return nil, errors.New("rolling window size must be positive")
	}
	return &RollingWindow{size: size, candles: make([]domain.Candle, 0, size)}, nil
}

func (w *RollingWindow) Add(candle domain.Candle) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.candles) == w.size {
		copy(w.candles, w.candles[1:])
		w.candles[len(w.candles)-1] = candle
		return
	}
	w.candles = append(w.candles, candle)
}

func (w *RollingWindow) Candles() []domain.Candle {
	w.mu.RLock()
	defer w.mu.RUnlock()
	result := make([]domain.Candle, len(w.candles))
	copy(result, w.candles)
	return result
}
