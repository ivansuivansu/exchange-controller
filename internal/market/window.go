package market

import (
	"errors"
	"sync"

	"github.com/ivansuivansu/exchange-controller/internal/domain"
)

type RollingWindow struct {
	mu     sync.RWMutex
	size   int
	events []domain.MarketEvent
}

func NewRollingWindow(size int) (*RollingWindow, error) {
	if size <= 0 {
		return nil, errors.New("rolling window size must be positive")
	}
	return &RollingWindow{size: size, events: make([]domain.MarketEvent, 0, size)}, nil
}

func (w *RollingWindow) Add(event domain.MarketEvent) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.events) == w.size {
		copy(w.events, w.events[1:])
		w.events[len(w.events)-1] = event
		return
	}
	w.events = append(w.events, event)
}

func (w *RollingWindow) Events() []domain.MarketEvent {
	w.mu.RLock()
	defer w.mu.RUnlock()
	result := make([]domain.MarketEvent, len(w.events))
	copy(result, w.events)
	return result
}
