package signal_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ivansuivansu/exchange-controller/internal/domain"
	"github.com/ivansuivansu/exchange-controller/internal/signal"
)

func detector(t *testing.T, cooldown time.Duration) *signal.DrawdownRecoveryDetector {
	t.Helper()
	d, err := signal.NewDrawdownRecoveryDetector(signal.DrawdownRecoveryConfig{
		WindowSize: 5, DrawdownThreshold: domain.MustDecimal("0.10"),
		RecoveryThreshold: domain.MustDecimal("0.05"), Cooldown: cooldown,
	})
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func feed(t *testing.T, d *signal.DrawdownRecoveryDetector, price string, at time.Time) (domain.Signal, error) {
	t.Helper()
	return d.Detect(context.Background(), domain.MarketEvent{
		Market: domain.Market{Base: "BTC", Quote: "USD", Instrument: "BTC_USD"},
		Price:  domain.MustDecimal(price), At: at,
	})
}

func TestDrawdownAndRecoveryDetection(t *testing.T) {
	d := detector(t, time.Minute)
	now := time.Unix(1000, 0)
	for i, price := range []string{"100", "98", "90"} {
		if _, err := feed(t, d, price, now.Add(time.Duration(i)*time.Second)); !errors.Is(err, signal.ErrNoSignal) {
			t.Fatalf("feed %s: %v", price, err)
		}
	}
	if d.State() != signal.StateDrawdown {
		t.Fatalf("state = %s", d.State())
	}
	if _, err := feed(t, d, "94", now.Add(3*time.Second)); !errors.Is(err, signal.ErrNoSignal) {
		t.Fatal(err)
	}
	if d.State() != signal.StateRecovering {
		t.Fatalf("state = %s", d.State())
	}
	result, err := feed(t, d, "94.5", now.Add(4*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if result.Price.String() != "94.5" || d.State() != signal.StateSignalEmitted {
		t.Fatalf("unexpected signal/state: %+v %s", result, d.State())
	}
}

func TestDuplicateSuppressionAndCooldownReset(t *testing.T) {
	d := detector(t, time.Minute)
	now := time.Unix(1000, 0)
	for i, price := range []string{"100", "90", "94.5"} {
		_, _ = feed(t, d, price, now.Add(time.Duration(i)*time.Second))
	}
	if _, err := feed(t, d, "96", now.Add(30*time.Second)); !errors.Is(err, signal.ErrNoSignal) {
		t.Fatalf("duplicate error = %v", err)
	}
	if d.State() != signal.StateSignalEmitted {
		t.Fatalf("state = %s", d.State())
	}
	if _, err := feed(t, d, "110", now.Add(62*time.Second)); !errors.Is(err, signal.ErrNoSignal) {
		t.Fatalf("reset error = %v", err)
	}
	if d.State() != signal.StateObserving {
		t.Fatalf("state after cooldown = %s", d.State())
	}
}
