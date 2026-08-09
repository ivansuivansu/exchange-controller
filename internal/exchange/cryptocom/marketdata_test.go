package cryptocom_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ivansuivansu/exchange-controller/internal/exchange/cryptocom"
)

func sourceFor(t *testing.T, server *httptest.Server, attempts int) *cryptocom.Source {
	t.Helper()
	source, err := cryptocom.NewSource(cryptocom.Config{
		BaseURL: server.URL, HTTPClient: server.Client(), Market: cryptocom.BTCUSD,
		MaxAttempts: attempts, RetryBackoff: time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	return source
}

func candleSourceFor(t *testing.T, server *httptest.Server, now time.Time) *cryptocom.Source {
	t.Helper()
	source, err := cryptocom.NewSource(cryptocom.Config{
		BaseURL: server.URL, HTTPClient: server.Client(), Market: cryptocom.BTCUSD,
		MaxAttempts: 1, RetryBackoff: time.Millisecond, CandleTimeframe: "M1",
		CandleCount: 10, Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}
	return source
}

func TestTickerResponseConversion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/public/get-tickers" || r.URL.Query().Get("instrument_name") != "BTC_USD" {
			t.Errorf("unexpected request URL: %s", r.URL)
		}
		_, _ = w.Write([]byte(`{"code":0,"result":{"data":[{"a":"64123.45","i":"BTC_USD","t":1700000000123}]}}`))
	}))
	defer server.Close()
	event, err := sourceFor(t, server, 1).Next(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if event.Market != cryptocom.BTCUSD || event.Price.String() != "64123.45" || event.At.UnixMilli() != 1700000000123 {
		t.Fatalf("unexpected event: %+v", event)
	}
}

func TestMalformedTickerResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`{"code":0,"result":`)) }))
	defer server.Close()
	if _, err := sourceFor(t, server, 1).Next(context.Background()); err == nil {
		t.Fatal("malformed response unexpectedly succeeded")
	}
}

func TestTickerHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "temporary", http.StatusServiceUnavailable)
	}))
	defer server.Close()
	if _, err := sourceFor(t, server, 1).Next(context.Background()); err == nil {
		t.Fatal("HTTP error unexpectedly succeeded")
	}
}

func TestTemporaryFailureIsRetried(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) < 3 {
			http.Error(w, "temporary", http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte(`{"code":0,"result":{"data":[{"a":"10","i":"BTC_USD","t":1700000000000}]}}`))
	}))
	defer server.Close()
	if _, err := sourceFor(t, server, 3).Next(context.Background()); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 3 {
		t.Fatalf("calls = %d, want 3", calls.Load())
	}
}

func TestSourceContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "temporary", http.StatusServiceUnavailable)
	}))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := sourceFor(t, server, 3).Next(ctx); err != context.Canceled {
		t.Fatalf("error = %v", err)
	}
}

func TestCandleConversionOrderingAndVolume(t *testing.T) {
	now := time.UnixMilli(1_700_000_180_000)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/public/get-candlestick" || r.URL.Query().Get("timeframe") != "M1" || r.URL.Query().Get("count") != "10" {
			t.Errorf("unexpected candle request: %s", r.URL)
		}
		_, _ = w.Write([]byte(`{"code":0,"result":{"data":[` +
			`{"o":"101","h":"103","l":"100","c":"102","v":"2.5","t":1700000060000},` +
			`{"o":"99","h":"102","l":"98","c":"101","v":"1.25","t":1700000000000}` +
			`]}}`))
	}))
	defer server.Close()
	source := candleSourceFor(t, server, now)
	first, err := source.NextCandle(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	second, err := source.NextCandle(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !first.OpenTime.Before(second.OpenTime) || first.Open.String() != "99" || second.Close.String() != "102" {
		t.Fatalf("candles not converted in chronological order: %+v %+v", first, second)
	}
	if !first.VolumeAvailable || first.Volume.String() != "1.25" || first.CloseTime.Sub(first.OpenTime) != time.Minute {
		t.Fatalf("unexpected candle conversion: %+v", first)
	}
}

func TestDuplicateCompletedCandlesAreSuppressedAcrossPolls(t *testing.T) {
	now := time.UnixMilli(1_700_000_180_000)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"code":0,"result":{"data":[{"o":"99","h":"102","l":"98","c":"101","v":"1","t":1700000000000}]}}`))
	}))
	defer server.Close()
	source := candleSourceFor(t, server, now)
	if _, err := source.NextCandle(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := source.NextCandle(context.Background()); !errors.Is(err, cryptocom.ErrNoCompletedCandle) {
		t.Fatalf("duplicate poll error = %v, want ErrNoCompletedCandle", err)
	}
}

func TestIncompleteCurrentCandleIsNotReturned(t *testing.T) {
	now := time.UnixMilli(1_700_000_180_000)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"code":0,"result":{"data":[{"o":"101","h":"103","l":"100","c":"102","v":"1","t":1700000180000}]}}`))
	}))
	defer server.Close()
	if _, err := candleSourceFor(t, server, now).NextCandle(context.Background()); !errors.Is(err, cryptocom.ErrNoCompletedCandle) {
		t.Fatalf("incomplete candle error = %v, want ErrNoCompletedCandle", err)
	}
}

func TestOverlappingPollingProducesSameHistoricalSequence(t *testing.T) {
	now := time.UnixMilli(1_700_000_300_000)
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		_, _ = w.Write([]byte(`{"code":0,"result":{"data":[` +
			`{"o":"100","h":"100","l":"99","c":"99","t":1700000000000},` +
			`{"o":"99","h":"99","l":"90","c":"91","t":1700000060000},` +
			`{"o":"91","h":"95","l":"91","c":"94.5","t":1700000120000}` +
			`]}}`))
	}))
	defer server.Close()
	source := candleSourceFor(t, server, now)
	var opens []int64
	for i := 0; i < 3; i++ {
		item, err := source.NextCandle(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		opens = append(opens, item.OpenTime.UnixMilli())
	}
	if _, err := source.NextCandle(context.Background()); !errors.Is(err, cryptocom.ErrNoCompletedCandle) {
		t.Fatal(err)
	}
	if calls.Load() != 2 || len(opens) != 3 || opens[0] >= opens[1] || opens[1] >= opens[2] {
		t.Fatalf("poll overlap changed sequence: calls=%d opens=%v", calls.Load(), opens)
	}
}

func TestHistoricalRangePaginationOrderingDeduplicationAndCompletion(t *testing.T) {
	from := time.UnixMilli(1_700_000_000_000).UTC()
	to := from.Add(4 * time.Minute)
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call := calls.Add(1)
		if r.URL.Query().Get("start_ts") == "" || r.URL.Query().Get("end_ts") == "" || r.URL.Query().Get("count") != "2" {
			t.Errorf("missing range query: %s", r.URL.RawQuery)
		}
		if call == 1 {
			_, _ = w.Write([]byte(`{"code":0,"result":{"data":[{"o":"101","h":"102","l":"100","c":"101","v":"1","t":1700000060000},{"o":"100","h":"101","l":"99","c":"100","v":"1","t":1700000000000}]}}`))
			return
		}
		_, _ = w.Write([]byte(`{"code":0,"result":{"data":[{"o":"102","h":"103","l":"101","c":"102","v":"1","t":1700000120000},{"o":"101","h":"102","l":"100","c":"101","v":"1","t":1700000060000},{"o":"103","h":"104","l":"102","c":"103","v":"1","t":1700000180000}]}}`))
	}))
	defer server.Close()
	source, err := cryptocom.NewSource(cryptocom.Config{BaseURL: server.URL, HTTPClient: server.Client(), Market: cryptocom.BTCUSD, MaxAttempts: 1, CandleTimeframe: "M1", CandleCount: 2, Now: func() time.Time { return to }})
	if err != nil {
		t.Fatal(err)
	}
	candles, err := source.LoadCompletedCandlesRange(context.Background(), from, to)
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 2 || len(candles) != 4 {
		t.Fatalf("calls=%d candles=%+v", calls.Load(), candles)
	}
	for i := 1; i < len(candles); i++ {
		if !candles[i-1].OpenTime.Before(candles[i].OpenTime) {
			t.Fatal("range is not chronological/deduplicated")
		}
	}
}
