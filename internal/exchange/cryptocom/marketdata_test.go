package cryptocom_test

import (
	"context"
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
