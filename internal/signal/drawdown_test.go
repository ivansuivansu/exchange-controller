package signal_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ivansuivansu/exchange-controller/internal/domain"
	"github.com/ivansuivansu/exchange-controller/internal/signal"
)

var testMarket = domain.Market{Base: "BTC", Quote: "USD", Instrument: "BTC_USD"}

func detector(t *testing.T, cooldown time.Duration) *signal.DrawdownRecoveryDetector {
	t.Helper()
	d, err := signal.NewDrawdownRecoveryDetector(signal.DrawdownRecoveryConfig{
		WindowSize: 5, DrawdownThreshold: domain.MustDecimal("0.10"),
		RecoveryThreshold: domain.MustDecimal("0.05"), Cooldown: cooldown,
	})
	if err != nil {
		t.Fatal(err)
	}
	d.SetClock(func() time.Time { return time.Unix(10_000, 0) })
	return d
}

func candle(openTime time.Time, high, low, closePrice string) domain.Candle {
	return domain.Candle{
		Market: testMarket, Open: domain.MustDecimal(closePrice), High: domain.MustDecimal(high),
		Low: domain.MustDecimal(low), Close: domain.MustDecimal(closePrice),
		OpenTime: openTime, CloseTime: openTime.Add(time.Minute),
	}
}

func TestDrawdownUsesHighAndTracksLocalLowThenRecovers(t *testing.T) {
	d := detector(t, time.Minute)
	now := time.Unix(1000, 0)
	sequence := []domain.Candle{
		candle(now, "100", "99", "99"),
		candle(now.Add(time.Minute), "99", "90", "91"),
	}
	for _, item := range sequence {
		if _, err := d.Detect(context.Background(), item); !errors.Is(err, signal.ErrNoSignal) {
			t.Fatal(err)
		}
	}
	if d.State() != signal.StateDrawdown {
		t.Fatalf("state = %s", d.State())
	}
	// A new lower wick becomes the local low and cannot signal on the same candle.
	if _, err := d.Detect(context.Background(), candle(now.Add(2*time.Minute), "92", "88", "92")); !errors.Is(err, signal.ErrNoSignal) {
		t.Fatal(err)
	}
	if d.State() != signal.StateDrawdown {
		t.Fatalf("state after new low = %s", d.State())
	}
	if _, err := d.Detect(context.Background(), candle(now.Add(3*time.Minute), "92", "89", "92")); !errors.Is(err, signal.ErrNoSignal) {
		t.Fatal(err)
	}
	if d.State() != signal.StateRecovering {
		t.Fatalf("state = %s", d.State())
	}
	result, err := d.Detect(context.Background(), candle(now.Add(4*time.Minute), "93", "90", "92.4"))
	if err != nil {
		t.Fatal(err)
	}
	if result.Price.String() != "92.4" || d.State() != signal.StateSignalEmitted {
		t.Fatalf("unexpected signal/state: %+v %s", result, d.State())
	}
	if result.Recovery == nil || result.Recovery.RecentHigh.String() != "100" || result.Recovery.LocalLow.String() != "88" ||
		result.Recovery.RecoveryPrice.String() != "92.4" || result.Recovery.DrawdownPercent.String() != "0.12" ||
		result.Recovery.RecoveryPercent.String() != "0.05" {
		t.Fatalf("structured recovery signal = %+v", result.Recovery)
	}
}

func TestDuplicateIncompleteAndCooldown(t *testing.T) {
	d := detector(t, time.Minute)
	now := time.Unix(1000, 0)
	first := candle(now, "100", "99", "99")
	if _, err := d.Detect(context.Background(), first); !errors.Is(err, signal.ErrNoSignal) {
		t.Fatal(err)
	}
	if _, err := d.Detect(context.Background(), first); !errors.Is(err, signal.ErrDuplicateCandle) {
		t.Fatalf("duplicate error = %v", err)
	}
	future := candle(time.Unix(20_000, 0), "100", "90", "95")
	if _, err := d.Detect(context.Background(), future); !errors.Is(err, signal.ErrIncompleteCandle) {
		t.Fatalf("incomplete error = %v", err)
	}
	for _, item := range []domain.Candle{
		candle(now.Add(time.Minute), "99", "90", "91"),
		candle(now.Add(2*time.Minute), "95", "91", "94.5"),
	} {
		_, _ = d.Detect(context.Background(), item)
	}
	if d.State() != signal.StateSignalEmitted {
		t.Fatalf("state = %s", d.State())
	}
	if _, err := d.Detect(context.Background(), candle(now.Add(150*time.Second), "96", "94", "95")); !errors.Is(err, signal.ErrNoSignal) {
		t.Fatal(err)
	}
	if _, err := d.Detect(context.Background(), candle(now.Add(4*time.Minute), "110", "109", "110")); !errors.Is(err, signal.ErrNoSignal) {
		t.Fatal(err)
	}
	if d.State() != signal.StateObserving {
		t.Fatalf("state after cooldown = %s", d.State())
	}
}

func TestPollingFrequencyDoesNotChangeHistoricalSignal(t *testing.T) {
	now := time.Unix(1000, 0)
	sequence := []domain.Candle{
		candle(now, "100", "99", "99"),
		candle(now.Add(time.Minute), "99", "90", "91"),
		candle(now.Add(2*time.Minute), "95", "91", "94.5"),
	}
	once := detector(t, time.Minute)
	withDuplicates := detector(t, time.Minute)
	var firstSignal, secondSignal domain.Signal
	for _, item := range sequence {
		result, err := once.Detect(context.Background(), item)
		if err == nil {
			firstSignal = result
		}
		result, err = withDuplicates.Detect(context.Background(), item)
		if err == nil {
			secondSignal = result
		}
		if _, duplicateErr := withDuplicates.Detect(context.Background(), item); !errors.Is(duplicateErr, signal.ErrDuplicateCandle) {
			t.Fatalf("duplicate candle error = %v", duplicateErr)
		}
	}
	if firstSignal.ID == "" || firstSignal.ID != secondSignal.ID || !firstSignal.Price.Equal(secondSignal.Price) {
		t.Fatalf("polling frequency changed signal: once=%+v duplicates=%+v", firstSignal, secondSignal)
	}
}
