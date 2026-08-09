package notifier

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ivansuivansu/exchange-controller/internal/domain"
	"github.com/ivansuivansu/exchange-controller/internal/market"
)

type Config struct {
	Market        domain.Market
	PollInterval  time.Duration
	WindowSize    int
	DropThreshold domain.Decimal
	Cooldown      time.Duration
}

type Snapshot struct {
	Config          Config
	CurrentPrice    domain.Decimal
	PreviousAverage domain.Decimal
	CurrentAverage  domain.Decimal
	CurrentChange   domain.Decimal
	Samples         int
	Ready           bool
	LastAlert       time.Time
}

type Alert struct {
	CurrentPrice    domain.Decimal
	PreviousAverage domain.Decimal
	CurrentAverage  domain.Decimal
	Change          domain.Decimal
	Threshold       domain.Decimal
	ComparisonTime  time.Duration
	At              time.Time
}

type Monitor struct {
	mu           sync.Mutex
	config       Config
	observations []domain.Decimal
	state        Snapshot
	armed        bool
	now          func() time.Time
}

func New(config Config) (*Monitor, error) {
	one := domain.MustDecimal("1")
	if config.Market.Base == "" || config.Market.Quote == "" || config.Market.Instrument == "" {
		return nil, errors.New("notifier market metadata is incomplete")
	}
	if config.PollInterval <= 0 || config.WindowSize <= 0 || config.Cooldown < 0 ||
		!config.DropThreshold.IsPositive() || !config.DropThreshold.Less(one) {
		return nil, errors.New("invalid notifier interval, window, threshold, or cooldown")
	}
	return &Monitor{config: config, armed: true, now: time.Now,
		state: Snapshot{Config: config}}, nil
}

func (m *Monitor) SetClock(now func() time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.now = now
}

func (m *Monitor) Observe(event domain.MarketEvent) (*Alert, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if event.Market != m.config.Market || !event.Price.IsPositive() {
		return nil, errors.New("notifier observation has invalid market or price")
	}
	total := 2 * m.config.WindowSize
	m.observations = append(m.observations, event.Price)
	if len(m.observations) > total {
		copy(m.observations, m.observations[len(m.observations)-total:])
		m.observations = m.observations[:total]
	}
	m.state.CurrentPrice = event.Price
	m.state.Samples = len(m.observations)
	if len(m.observations) < total {
		return nil, nil
	}
	previous, err := average(m.observations[:m.config.WindowSize])
	if err != nil {
		return nil, err
	}
	current, err := average(m.observations[m.config.WindowSize:])
	if err != nil {
		return nil, err
	}
	difference, err := current.Sub(previous)
	if err != nil {
		return nil, err
	}
	change, err := difference.Div(previous, domain.RoundTowardZero)
	if err != nil {
		return nil, err
	}
	m.state.PreviousAverage, m.state.CurrentAverage = previous, current
	m.state.CurrentChange, m.state.Ready = change, true
	negativeThreshold, _ := domain.Decimal{}.Sub(m.config.DropThreshold)
	declining := change.Less(negativeThreshold) || change.Equal(negativeThreshold)
	if !declining && !m.armed {
		// Require meaningful recovery before re-arming. This hysteresis avoids
		// repeated alerts when averages hover immediately around the boundary.
		half, _ := m.config.DropThreshold.Div(domain.MustDecimal("2"), domain.RoundTowardZero)
		rearmAt, _ := domain.Decimal{}.Sub(half)
		if !change.Less(rearmAt) {
			m.armed = true
		}
	}
	now := m.now().UTC()
	if !declining || !m.armed || (!m.state.LastAlert.IsZero() && now.Sub(m.state.LastAlert) < m.config.Cooldown) {
		return nil, nil
	}
	m.armed = false
	m.state.LastAlert = now
	return &Alert{CurrentPrice: event.Price, PreviousAverage: previous, CurrentAverage: current,
		Change: change, Threshold: m.config.DropThreshold,
		ComparisonTime: time.Duration(total) * m.config.PollInterval, At: now}, nil
}

func average(values []domain.Decimal) (domain.Decimal, error) {
	if len(values) == 0 {
		return domain.Decimal{}, errors.New("cannot average an empty window")
	}
	var sum domain.Decimal
	var err error
	for _, value := range values {
		sum, err = sum.Add(value)
		if err != nil {
			return domain.Decimal{}, err
		}
	}
	return sum.Div(domain.MustDecimal(fmt.Sprintf("%d", len(values))), domain.RoundTowardZero)
}

func (m *Monitor) Snapshot() Snapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.state
}

type TextSender interface {
	SendText(context.Context, int64, string) error
}

type Service struct {
	Source  market.MarketDataSource
	Monitor *Monitor
	Sender  TextSender
	ChatID  int64
}

func (s Service) Run(ctx context.Context) error {
	if s.Source == nil || s.Monitor == nil || s.Sender == nil || s.ChatID == 0 {
		return errors.New("notifier service dependencies are incomplete")
	}
	for {
		event, err := s.Source.Next(ctx)
		if err != nil {
			return err
		}
		alert, err := s.Monitor.Observe(event)
		if err != nil {
			return err
		}
		if alert != nil {
			if err := s.Sender.SendText(ctx, s.ChatID, RenderAlert(*alert)); err != nil {
				return err
			}
		}
	}
}

func RenderAlert(alert Alert) string {
	return fmt.Sprintf("📉 BTC price decline detected\n\nCurrent:       $%s\nPrevious avg:  $%s\nCurrent avg:   $%s\n\nAverage change: %s\nThreshold:      -%s\nWindow:         ~%s\n\nINFORMATION ONLY\nNo trade was placed.",
		money(alert.CurrentPrice), money(alert.PreviousAverage), money(alert.CurrentAverage),
		Percent(alert.Change), Percent(alert.Threshold), approximate(alert.ComparisonTime))
}

func Percent(value domain.Decimal) string {
	percentage, err := value.Mul(domain.MustDecimal("100"), domain.RoundTowardZero)
	if err != nil {
		return value.String() + "%"
	}
	return percentage.String() + "%"
}

func money(value domain.Decimal) string {
	parts := strings.SplitN(value.String(), ".", 2)
	whole := parts[0]
	sign := ""
	if strings.HasPrefix(whole, "-") {
		sign, whole = "-", whole[1:]
	}
	for i := len(whole) - 3; i > 0; i -= 3 {
		whole = whole[:i] + "," + whole[i:]
	}
	if len(parts) == 2 {
		return sign + whole + "." + parts[1]
	}
	return sign + whole
}

func approximate(duration time.Duration) string {
	if duration%time.Minute == 0 {
		minutes := int(duration / time.Minute)
		unit := "minutes"
		if minutes == 1 {
			unit = "minute"
		}
		return fmt.Sprintf("%d %s", minutes, unit)
	}
	return duration.String()
}
