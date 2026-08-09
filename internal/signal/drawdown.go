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

var (
	ErrNoSignal         = errors.New("no signal")
	ErrDuplicateCandle  = errors.New("candle was already processed")
	ErrIncompleteCandle = errors.New("candle is not complete")
)

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
	mu           sync.Mutex
	config       DrawdownRecoveryConfig
	window       *market.RollingWindow
	state        DetectorState
	high         domain.Decimal
	low          domain.Decimal
	emittedAt    time.Time
	lastOpenTime time.Time
	now          func() time.Time
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
	return &DrawdownRecoveryDetector{config: config, window: window, state: StateObserving, now: time.Now}, nil
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

func (d *DrawdownRecoveryDetector) SetClock(now func() time.Time) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.now = now
}

func (d *DrawdownRecoveryDetector) Detect(ctx context.Context, candle domain.Candle) (domain.Signal, error) {
	if err := ctx.Err(); err != nil {
		return domain.Signal{}, err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if candle.OpenTime.IsZero() || !candle.OpenTime.After(d.lastOpenTime) {
		return domain.Signal{}, ErrDuplicateCandle
	}
	if candle.CloseTime.After(d.now()) {
		return domain.Signal{}, ErrIncompleteCandle
	}
	d.lastOpenTime = candle.OpenTime
	if d.state == StateSignalEmitted {
		if candle.CloseTime.Sub(d.emittedAt) < d.config.Cooldown {
			return domain.Signal{}, ErrNoSignal
		}
		d.window, _ = market.NewRollingWindow(d.config.WindowSize)
		d.state, d.high, d.low = StateObserving, domain.Decimal{}, domain.Decimal{}
	}
	d.window.Add(candle)

	switch d.state {
	case StateObserving:
		d.high = windowHigh(d.window.Candles())
		if reached(d.high, candle.Low, d.config.DrawdownThreshold, false) {
			d.low, d.state = candle.Low, StateDrawdown
		}
	case StateDrawdown, StateRecovering:
		if candle.Low.Less(d.low) {
			d.low, d.state = candle.Low, StateDrawdown
			return domain.Signal{}, ErrNoSignal
		}
		d.state = StateRecovering
		if reached(d.low, candle.Close, d.config.RecoveryThreshold, true) {
			d.state, d.emittedAt = StateSignalEmitted, candle.CloseTime
			return domain.Signal{
				ID: fmt.Sprintf("drawdown-recovery-%d", candle.OpenTime.UnixNano()), Market: candle.Market,
				ObservedAt: candle.CloseTime, Price: candle.Close,
				Description: fmt.Sprintf("recovery from local low %s after drawdown from %s", d.low, d.high),
			}, nil
		}
	}
	return domain.Signal{}, ErrNoSignal
}

func windowHigh(candles []domain.Candle) domain.Decimal {
	var high domain.Decimal
	for i, candle := range candles {
		if i == 0 || high.Less(candle.High) {
			high = candle.High
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
	return new(big.Rat).Quo(delta, fromRat).Cmp(thresholdRat) >= 0
}
