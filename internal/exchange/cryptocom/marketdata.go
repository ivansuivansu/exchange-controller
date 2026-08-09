package cryptocom

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"time"

	"github.com/ivansuivansu/exchange-controller/internal/domain"
	"github.com/ivansuivansu/exchange-controller/internal/market"
)

const DefaultBaseURL = "https://api.crypto.com/exchange/v1"

var BTCUSD = domain.Market{Base: "BTC", Quote: "USD", Instrument: "BTC_USD"}

type Config struct {
	BaseURL         string
	HTTPClient      *http.Client
	Market          domain.Market
	PollInterval    time.Duration
	MaxAttempts     int
	RetryBackoff    time.Duration
	CandleTimeframe string
	CandleCount     int
	Now             func() time.Time
}

type Source struct {
	baseURL         string
	client          *http.Client
	market          domain.Market
	pollInterval    time.Duration
	maxAttempts     int
	retryBackoff    time.Duration
	tickerPolled    bool
	candlePolled    bool
	candleTimeframe string
	candleDuration  time.Duration
	candleCount     int
	now             func() time.Time
	pendingCandles  []domain.Candle
	lastCandleOpen  time.Time
}

var _ market.MarketDataSource = (*Source)(nil)
var _ market.CandleDataSource = (*Source)(nil)

var ErrNoCompletedCandle = errors.New("Crypto.com response has no new completed candle")

func NewSource(config Config) (*Source, error) {
	if config.BaseURL == "" {
		config.BaseURL = DefaultBaseURL
	}
	if _, err := url.ParseRequestURI(config.BaseURL); err != nil {
		return nil, fmt.Errorf("Crypto.com base URL: %w", err)
	}
	if config.HTTPClient == nil {
		config.HTTPClient = &http.Client{Timeout: 10 * time.Second}
	}
	if config.Market.Base == "" || config.Market.Quote == "" || config.Market.Instrument == "" {
		return nil, errors.New("complete market metadata is required")
	}
	if config.MaxAttempts <= 0 {
		config.MaxAttempts = 3
	}
	if config.RetryBackoff <= 0 {
		config.RetryBackoff = 250 * time.Millisecond
	}
	if config.CandleTimeframe == "" {
		config.CandleTimeframe = "M1"
	}
	candleDuration, err := TimeframeDuration(config.CandleTimeframe)
	if err != nil {
		return nil, err
	}
	if config.CandleCount <= 0 {
		config.CandleCount = 25
	}
	if config.Now == nil {
		config.Now = time.Now
	}
	return &Source{baseURL: config.BaseURL, client: config.HTTPClient, market: config.Market,
		pollInterval: config.PollInterval, maxAttempts: config.MaxAttempts, retryBackoff: config.RetryBackoff,
		candleTimeframe: config.CandleTimeframe, candleDuration: candleDuration,
		candleCount: config.CandleCount, now: config.Now}, nil
}

func (s *Source) Next(ctx context.Context) (domain.MarketEvent, error) {
	if s.tickerPolled && s.pollInterval > 0 {
		if err := wait(ctx, s.pollInterval); err != nil {
			return domain.MarketEvent{}, err
		}
	}
	s.tickerPolled = true
	var lastErr error
	for attempt := 0; attempt < s.maxAttempts; attempt++ {
		event, err := s.fetch(ctx)
		if err == nil {
			return event, nil
		}
		lastErr = err
		if attempt+1 < s.maxAttempts {
			if err := wait(ctx, s.retryBackoff*time.Duration(attempt+1)); err != nil {
				return domain.MarketEvent{}, err
			}
		}
	}
	return domain.MarketEvent{}, fmt.Errorf("Crypto.com market data after %d attempts: %w", s.maxAttempts, lastErr)
}

func TimeframeDuration(timeframe string) (time.Duration, error) {
	durations := map[string]time.Duration{
		"M1": time.Minute, "M5": 5 * time.Minute, "M15": 15 * time.Minute,
		"M30": 30 * time.Minute, "H1": time.Hour, "H2": 2 * time.Hour,
		"H4": 4 * time.Hour, "H12": 12 * time.Hour, "D1": 24 * time.Hour,
	}
	duration, ok := durations[timeframe]
	if !ok {
		return 0, fmt.Errorf("unsupported Crypto.com candle timeframe %q", timeframe)
	}
	return duration, nil
}

// LoadCompletedCandles downloads a historical batch. Replay state and
// iteration are intentionally owned by the backtest package, not this loader.
func (s *Source) LoadCompletedCandles(ctx context.Context) ([]domain.Candle, error) {
	var lastErr error
	for attempt := 0; attempt < s.maxAttempts; attempt++ {
		candles, err := s.fetchCandles(ctx)
		if err == nil {
			completed := make([]domain.Candle, 0, len(candles))
			var last time.Time
			for _, candle := range candles {
				if candle.CloseTime.After(s.now()) || !candle.OpenTime.After(last) {
					continue
				}
				completed = append(completed, candle)
				last = candle.OpenTime
			}
			if len(completed) > 0 {
				return completed, nil
			}
			err = ErrNoCompletedCandle
		}
		lastErr = err
		if attempt+1 < s.maxAttempts {
			if err := wait(ctx, s.retryBackoff*time.Duration(attempt+1)); err != nil {
				return nil, err
			}
		}
	}
	return nil, fmt.Errorf("Crypto.com historical candles after %d attempts: %w", s.maxAttempts, lastErr)
}

func (s *Source) NextCandle(ctx context.Context) (domain.Candle, error) {
	if len(s.pendingCandles) > 0 {
		return s.popCandle(), nil
	}
	if s.candlePolled && s.pollInterval > 0 {
		if err := wait(ctx, s.pollInterval); err != nil {
			return domain.Candle{}, err
		}
	}
	s.candlePolled = true
	var lastErr error
	for attempt := 0; attempt < s.maxAttempts; attempt++ {
		candles, err := s.fetchCandles(ctx)
		if err == nil {
			newestQueued := s.lastCandleOpen
			for _, candle := range candles {
				if candle.OpenTime.After(newestQueued) && !candle.CloseTime.After(s.now()) {
					s.pendingCandles = append(s.pendingCandles, candle)
					newestQueued = candle.OpenTime
				}
			}
			if len(s.pendingCandles) > 0 {
				return s.popCandle(), nil
			}
			err = ErrNoCompletedCandle
		}
		lastErr = err
		if attempt+1 < s.maxAttempts {
			if err := wait(ctx, s.retryBackoff*time.Duration(attempt+1)); err != nil {
				return domain.Candle{}, err
			}
		}
	}
	return domain.Candle{}, fmt.Errorf("Crypto.com candle data after %d attempts: %w", s.maxAttempts, lastErr)
}

func (s *Source) popCandle() domain.Candle {
	candle := s.pendingCandles[0]
	s.pendingCandles = s.pendingCandles[1:]
	s.lastCandleOpen = candle.OpenTime
	return candle
}

func wait(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

type tickerResponse struct {
	Code   int `json:"code"`
	Result struct {
		Data []ticker `json:"data"`
	} `json:"result"`
}

type ticker struct {
	Price      *string `json:"a"`
	Instrument string  `json:"i"`
	Timestamp  int64   `json:"t"`
}

type candlestickResponse struct {
	Code   int `json:"code"`
	Result struct {
		Data []candlestick `json:"data"`
	} `json:"result"`
}

type candlestick struct {
	Open      string  `json:"o"`
	High      string  `json:"h"`
	Low       string  `json:"l"`
	Close     string  `json:"c"`
	Volume    *string `json:"v"`
	Timestamp int64   `json:"t"`
}

func (s *Source) fetchCandles(ctx context.Context) ([]domain.Candle, error) {
	endpoint, err := url.Parse(s.baseURL + "/public/get-candlestick")
	if err != nil {
		return nil, err
	}
	query := endpoint.Query()
	query.Set("instrument_name", string(s.market.Instrument))
	query.Set("timeframe", s.candleTimeframe)
	query.Set("count", strconv.Itoa(s.candleCount))
	endpoint.RawQuery = query.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, err
	}
	response, err := s.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("Crypto.com HTTP status %d", response.StatusCode)
	}
	var payload candlestickResponse
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode Crypto.com candles: %w", err)
	}
	if payload.Code != 0 {
		return nil, fmt.Errorf("Crypto.com API code %d", payload.Code)
	}
	candles := make([]domain.Candle, 0, len(payload.Result.Data))
	for _, item := range payload.Result.Data {
		open, err := domain.ParseDecimal(item.Open)
		if err != nil {
			return nil, fmt.Errorf("invalid candle open: %w", err)
		}
		high, err := domain.ParseDecimal(item.High)
		if err != nil {
			return nil, fmt.Errorf("invalid candle high: %w", err)
		}
		low, err := domain.ParseDecimal(item.Low)
		if err != nil {
			return nil, fmt.Errorf("invalid candle low: %w", err)
		}
		closePrice, err := domain.ParseDecimal(item.Close)
		if err != nil {
			return nil, fmt.Errorf("invalid candle close: %w", err)
		}
		if item.Timestamp <= 0 || !open.IsPositive() || !high.IsPositive() || !low.IsPositive() || !closePrice.IsPositive() {
			return nil, errors.New("invalid Crypto.com candle")
		}
		openTime := time.UnixMilli(item.Timestamp).UTC()
		candle := domain.Candle{Market: s.market, Open: open, High: high, Low: low, Close: closePrice,
			OpenTime: openTime, CloseTime: openTime.Add(s.candleDuration)}
		if item.Volume != nil {
			volume, err := domain.ParseDecimal(*item.Volume)
			if err != nil {
				return nil, fmt.Errorf("invalid candle volume: %w", err)
			}
			candle.Volume, candle.VolumeAvailable = volume, true
		}
		candles = append(candles, candle)
	}
	sort.Slice(candles, func(i, j int) bool { return candles[i].OpenTime.Before(candles[j].OpenTime) })
	return candles, nil
}

func (s *Source) fetch(ctx context.Context) (domain.MarketEvent, error) {
	endpoint, err := url.Parse(s.baseURL + "/public/get-tickers")
	if err != nil {
		return domain.MarketEvent{}, err
	}
	query := endpoint.Query()
	query.Set("instrument_name", string(s.market.Instrument))
	endpoint.RawQuery = query.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return domain.MarketEvent{}, err
	}
	response, err := s.client.Do(request)
	if err != nil {
		return domain.MarketEvent{}, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		return domain.MarketEvent{}, fmt.Errorf("Crypto.com HTTP status %d", response.StatusCode)
	}
	var payload tickerResponse
	decoder := json.NewDecoder(io.LimitReader(response.Body, 1<<20))
	if err := decoder.Decode(&payload); err != nil {
		return domain.MarketEvent{}, fmt.Errorf("decode Crypto.com ticker: %w", err)
	}
	if payload.Code != 0 {
		return domain.MarketEvent{}, fmt.Errorf("Crypto.com API code %d", payload.Code)
	}
	for _, item := range payload.Result.Data {
		if item.Instrument != string(s.market.Instrument) {
			continue
		}
		if item.Price == nil {
			return domain.MarketEvent{}, errors.New("Crypto.com ticker has no latest price")
		}
		price, err := domain.ParseDecimal(*item.Price)
		if err != nil || !price.IsPositive() {
			return domain.MarketEvent{}, errors.New("Crypto.com ticker has invalid latest price")
		}
		if item.Timestamp <= 0 {
			return domain.MarketEvent{}, errors.New("Crypto.com ticker has invalid timestamp")
		}
		return domain.MarketEvent{Market: s.market, Price: price, At: time.UnixMilli(item.Timestamp).UTC()}, nil
	}
	return domain.MarketEvent{}, errors.New("Crypto.com response does not contain requested instrument")
}
