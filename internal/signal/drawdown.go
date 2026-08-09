package signal

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/ivansuivansu/exchange-controller/internal/domain"
	"github.com/ivansuivansu/exchange-controller/internal/market"
)

var ErrNoSignal = errors.New("no signal")

type DetectorState string

const (
	StateObserving     DetectorState = "observing"
	StateDrawdown      DetectorState = "drawdown"
	StateRecovering    DetectorState = "recovering"
	StateSignalEmitted DetectorState = "signal_emitted"
)

type DrawdownRecoveryConfig struct {
	WindowSize        int
	DrawdownThreshold domain.Decimal
	RecoveryThreshold domain.Decimal
	Cooldown          time.Duration
}

type DrawdownRecoveryDetector struct {
	mu        sync.Mutex
	config    DrawdownRecoveryConfig
	window    *market.RollingWindow
	state     DetectorState
	high      domain.Decimal
	low       domain.Decimal
	emittedAt time.Time
}

var _ SignalDetector = (*DrawdownRecoveryDetector)(nil)

func NewDrawdownRecoveryDetector(config DrawdownRecoveryConfig) (*DrawdownRecoveryDetector, error) {
	if config.WindowSize <= 1 {
		return nil, errors.New("signal window size must be greater than one")
	}
	if err := validateRatio(config.DrawdownThreshold); err != nil {
		return nil, fmt.Errorf("drawdown threshold: %w", err)
	}
	if err := validateRatio(config.RecoveryThreshold); err != nil {
		return nil, fmt.Errorf("recovery threshold: %w", err)
	}
	if config.Cooldown < 0 {
		return nil, errors.New("signal cooldown must not be negative")
	}
	window, _ := market.NewRollingWindow(config.WindowSize)
	return &DrawdownRecoveryDetector{config: config, window: window, state: StateObserving}, nil
}

func validateRatio(value domain.Decimal) error {
	if !value.IsPositive() || !value.Less(domain.MustDecimal("1")) {
		return errors.New("must be greater than zero and less than one")
	}
	return nil
}

func (d *DrawdownRecoveryDetector) State() DetectorState {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.state
}

func (d *DrawdownRecoveryDetector) Detect(ctx context.Context, event domain.MarketEvent) (domain.Signal, error) {
	if err := ctx.Err(); err != nil {
		return domain.Signal{}, err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.state == StateSignalEmitted {
		if event.At.Sub(d.emittedAt) < d.config.Cooldown {
			return domain.Signal{}, ErrNoSignal
		}
		d.window, _ = market.NewRollingWindow(d.config.WindowSize)
		d.state, d.high, d.low = StateObserving, domain.Decimal{}, domain.Decimal{}
	}
	d.window.Add(event)

	switch d.state {
	case StateObserving:
		d.high = windowHigh(d.window.Events())
		if reached(d.high, event.Price, d.config.DrawdownThreshold, false) {
			d.low = event.Price
			d.state = StateDrawdown
		}
	case StateDrawdown, StateRecovering:
		if event.Price.Less(d.low) || event.Price.Equal(d.low) {
			d.low, d.state = event.Price, StateDrawdown
			return domain.Signal{}, ErrNoSignal
		}
		d.state = StateRecovering
		if reached(d.low, event.Price, d.config.RecoveryThreshold, true) {
			d.state, d.emittedAt = StateSignalEmitted, event.At
			return domain.Signal{
				ID: fmt.Sprintf("drawdown-recovery-%d", event.At.UnixNano()), Market: event.Market,
				ObservedAt: event.At, Price: event.Price,
				Description: fmt.Sprintf("recovery from local low %s after drawdown from %s", d.low, d.high),
			}, nil
		}
	}
	return domain.Signal{}, ErrNoSignal
}

func windowHigh(events []domain.MarketEvent) domain.Decimal {
	var high domain.Decimal
	for i, event := range events {
		if i == 0 || high.Less(event.Price) {
			high = event.Price
		}
	}
	return high
}

func reached(from, to, threshold domain.Decimal, upward bool) bool {
	fromRat, _ := new(big.Rat).SetString(from.String())
	toRat, _ := new(big.Rat).SetString(to.String())
	thresholdRat, _ := new(big.Rat).SetString(threshold.String())
	delta := new(big.Rat)
	if upward {
		delta.Sub(toRat, fromRat)
	} else {
		delta.Sub(fromRat, toRat)
	}
	if delta.Sign() < 0 {
		return false
	}
	ratio := new(big.Rat).Quo(delta, fromRat)
	return ratio.Cmp(thresholdRat) >= 0
}
