package notifier

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/ivansuivansu/exchange-controller/internal/domain"
)

var btcusd = domain.Market{Base: "BTC", Quote: "USD", Instrument: "BTC_USD"}

func newMonitor(t *testing.T, window int, threshold string, cooldown time.Duration, now *time.Time) *Monitor {
	t.Helper()
	m, err := New(Config{Market: btcusd, PollInterval: 10 * time.Second, WindowSize: window,
		DropThreshold: domain.MustDecimal(threshold), Cooldown: cooldown})
	if err != nil {
		t.Fatal(err)
	}
	m.SetClock(func() time.Time { return *now })
	return m
}

func observe(t *testing.T, m *Monitor, prices ...string) []*Alert {
	t.Helper()
	alerts := make([]*Alert, 0, len(prices))
	for _, price := range prices {
		alert, err := m.Observe(domain.MarketEvent{Market: btcusd, Price: domain.MustDecimal(price)})
		if err != nil {
			t.Fatal(err)
		}
		alerts = append(alerts, alert)
	}
	return alerts
}

func TestStableAndRisingPricesDoNotAlert(t *testing.T) {
	now := time.Unix(1000, 0)
	for name, prices := range map[string][]string{
		"stable": {"100", "100", "100", "100", "100", "100", "100", "100", "100", "100", "100", "100"},
		"rising": {"100", "100", "100", "100", "100", "100", "101", "101", "101", "101", "101", "101"},
	} {
		t.Run(name, func(t *testing.T) {
			for _, alert := range observe(t, newMonitor(t, 6, "0.0036", 5*time.Minute, &now), prices...) {
				if alert != nil {
					t.Fatal("unexpected alert")
				}
			}
		})
	}
}

func TestDeclineThresholdBoundary(t *testing.T) {
	now := time.Unix(1000, 0)
	tests := []struct {
		name, current string
		want          bool
	}{
		{"below threshold", "99.65", false},
		{"exact threshold", "99.64", true},
		{"greater decline", "99.5", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prices := []string{"100", "100", "100", "100", "100", "100", tt.current, tt.current, tt.current, tt.current, tt.current, tt.current}
			alerts := observe(t, newMonitor(t, 6, "0.0036", 5*time.Minute, &now), prices...)
			if got := alerts[len(alerts)-1] != nil; got != tt.want {
				t.Fatalf("alert=%v, want %v", got, tt.want)
			}
		})
	}
}

func TestDecimalAverageAndSlidingWindow(t *testing.T) {
	now := time.Unix(1000, 0)
	m := newMonitor(t, 2, "0.0036", 0, &now)
	observe(t, m, "100.1", "100.3", "99.7", "99.9")
	state := m.Snapshot()
	if state.PreviousAverage.String() != "100.2" || state.CurrentAverage.String() != "99.8" || state.CurrentChange.String() != "-0.00399201" {
		t.Fatalf("unexpected decimal averages: %+v", state)
	}
	observe(t, m, "100.5")
	state = m.Snapshot()
	if state.PreviousAverage.String() != "100" || state.CurrentAverage.String() != "100.2" {
		t.Fatalf("window did not slide one observation: %+v", state)
	}
}

func TestNoDuplicateAlertAndResetAfterRecovery(t *testing.T) {
	now := time.Unix(1000, 0)
	m := newMonitor(t, 1, "0.005", 5*time.Minute, &now)
	if alerts := observe(t, m, "100", "99"); alerts[1] == nil {
		t.Fatal("initial alert missing")
	}
	now = now.Add(10 * time.Minute)
	if observe(t, m, "98")[0] != nil {
		t.Fatal("continuous decline produced duplicate alert")
	}
	if observe(t, m, "100")[0] != nil {
		t.Fatal("recovery produced alert")
	}
	if observe(t, m, "99")[0] == nil {
		t.Fatal("recovered monitor did not alert on a new decline")
	}
}

func TestCooldownBlocksNewDeclineUntilElapsed(t *testing.T) {
	now := time.Unix(1000, 0)
	m := newMonitor(t, 1, "0.005", 5*time.Minute, &now)
	observe(t, m, "100", "99")
	observe(t, m, "101") // recover and re-arm
	now = now.Add(time.Minute)
	if observe(t, m, "100")[0] != nil {
		t.Fatal("alert bypassed cooldown")
	}
	now = now.Add(5 * time.Minute)
	if observe(t, m, "99")[0] == nil {
		t.Fatal("alert missing after cooldown")
	}
}

func TestHumanRenderingAndStatus(t *testing.T) {
	if got := Percent(domain.MustDecimal("-0.0036")); got != "-0.36%" {
		t.Fatalf("percentage=%q", got)
	}
	alert := RenderAlert(Alert{CurrentPrice: domain.MustDecimal("64820"), PreviousAverage: domain.MustDecimal("65080"), CurrentAverage: domain.MustDecimal("64840"), Change: domain.MustDecimal("-0.0037"), Threshold: domain.MustDecimal("0.0036"), ComparisonTime: 2 * time.Minute})
	for _, text := range []string{"$64,820", "-0.37%", "-0.36%", "~2 minutes", "INFORMATION ONLY", "No trade was placed"} {
		if !strings.Contains(alert, text) {
			t.Errorf("alert missing %q: %s", text, alert)
		}
	}
	now := time.Unix(1000, 0)
	m := newMonitor(t, 6, "0.0036", 5*time.Minute, &now)
	status := RenderStatus(m.Snapshot())
	for _, text := range []string{"Notifier: ON", "Instrument: BTC_USD", "Poll interval: 10s", "Average window: 6 samples (~1 minute)", "Comparison period: ~2 minutes", "Drop threshold: 0.36%", "Cooldown: 5m0s", "Last alert: never"} {
		if !strings.Contains(status, text) {
			t.Errorf("status missing %q: %s", text, status)
		}
	}
}

type fakeSender struct{ messages []string }

func (s *fakeSender) SendText(_ context.Context, _ int64, text string) error {
	s.messages = append(s.messages, text)
	return nil
}

func TestCommandAuthorization(t *testing.T) {
	now := time.Unix(1000, 0)
	m := newMonitor(t, 6, "0.0036", 5*time.Minute, &now)
	sender := &fakeSender{}
	h := NewCommandHandler(m, sender, []int64{1}, []int64{2})
	if _, err := h.HandleCommand(context.Background(), 9, 2, "/status"); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("unauthorized error=%v", err)
	}
	if _, err := h.HandleCommand(context.Background(), 1, 2, "/status"); err != nil {
		t.Fatal(err)
	}
	if len(sender.messages) != 1 || !strings.Contains(sender.messages[0], "Notifier: ON") {
		t.Fatalf("messages=%v", sender.messages)
	}
}
