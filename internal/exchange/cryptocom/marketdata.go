package cryptocom

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/ivansuivansu/exchange-controller/internal/domain"
	"github.com/ivansuivansu/exchange-controller/internal/market"
)

const DefaultBaseURL = "https://api.crypto.com/exchange/v1"

var BTCUSD = domain.Market{Base: "BTC", Quote: "USD", Instrument: "BTC_USD"}

type Config struct {
	BaseURL      string
	HTTPClient   *http.Client
	Market       domain.Market
	PollInterval time.Duration
	MaxAttempts  int
	RetryBackoff time.Duration
}

type Source struct {
	baseURL      string
	client       *http.Client
	market       domain.Market
	pollInterval time.Duration
	maxAttempts  int
	retryBackoff time.Duration
	polled       bool
}

var _ market.MarketDataSource = (*Source)(nil)

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
	return &Source{baseURL: config.BaseURL, client: config.HTTPClient, market: config.Market,
		pollInterval: config.PollInterval, maxAttempts: config.MaxAttempts, retryBackoff: config.RetryBackoff}, nil
}

func (s *Source) Next(ctx context.Context) (domain.MarketEvent, error) {
	if s.polled && s.pollInterval > 0 {
		if err := wait(ctx, s.pollInterval); err != nil {
			return domain.MarketEvent{}, err
		}
	}
	s.polled = true
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
