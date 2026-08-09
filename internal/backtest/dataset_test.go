package backtest

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/ivansuivansu/exchange-controller/internal/domain"
)

func TestCandleCSVRoundTripOrderingDeduplicationAndRange(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	first := domain.Candle{Market: btcusd, Open: domain.MustDecimal("100"), High: domain.MustDecimal("102"), Low: domain.MustDecimal("99"), Close: domain.MustDecimal("101"), Volume: domain.MustDecimal("2.5"), VolumeAvailable: true, OpenTime: start, CloseTime: start.Add(time.Minute)}
	second := first
	second.OpenTime, second.CloseTime = start.Add(time.Minute), start.Add(2*time.Minute)
	second.Open, second.Close = domain.MustDecimal("101"), domain.MustDecimal("102")
	var encoded bytes.Buffer
	if err := WriteCandleCSV(&encoded, []domain.Candle{second, first, first}); err != nil {
		t.Fatal(err)
	}
	loaded, err := ReadCandleCSV(strings.NewReader(encoded.String()), btcusd, time.Minute, start, start.Add(2*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 2 || loaded[0].OpenTime != start || loaded[1].OpenTime != start.Add(time.Minute) || !loaded[0].VolumeAvailable {
		t.Fatalf("unexpected roundtrip: %+v", loaded)
	}
	filtered, err := ReadCandleCSV(strings.NewReader(encoded.String()), btcusd, time.Minute, start.Add(time.Minute), start.Add(2*time.Minute))
	if err != nil || len(filtered) != 1 || filtered[0].OpenTime != second.OpenTime {
		t.Fatalf("range result=%+v err=%v", filtered, err)
	}
}

func TestReadCandleCSVRejectsMalformedData(t *testing.T) {
	data := "timestamp,open,high,low,close,volume\n2025-01-01T00:00:00Z,100,nope,99,101,1\n"
	if _, err := ReadCandleCSV(strings.NewReader(data), btcusd, time.Minute, time.Time{}, time.Time{}); err == nil {
		t.Fatal("invalid CSV accepted")
	}
}

func TestNormalizeCandlesRejectsConflictingDuplicate(t *testing.T) {
	candles := historicalSequence(1)[:1]
	other := candles[0]
	other.Close = domain.MustDecimal("99.5")
	if _, err := NormalizeCandles([]domain.Candle{candles[0], other}, btcusd, time.Minute, time.Time{}, time.Time{}); err == nil {
		t.Fatal("conflicting duplicate accepted")
	}
}
