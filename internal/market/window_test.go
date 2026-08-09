package market_test

import (
	"testing"
	"time"

	"github.com/ivansuivansu/exchange-controller/internal/domain"
	"github.com/ivansuivansu/exchange-controller/internal/market"
)

func TestRollingCandleWindow(t *testing.T) {
	window, err := market.NewRollingWindow(3)
	if err != nil {
		t.Fatal(err)
	}
	for i, price := range []string{"1", "2", "3", "4"} {
		window.Add(domain.Candle{Close: domain.MustDecimal(price), OpenTime: time.Unix(int64(i), 0)})
	}
	candles := window.Candles()
	if len(candles) != 3 || candles[0].Close.String() != "2" || candles[2].Close.String() != "4" {
		t.Fatalf("unexpected window: %+v", candles)
	}
	candles[0].Close = domain.MustDecimal("99")
	if window.Candles()[0].Close.String() != "2" {
		t.Fatal("Candles exposed internal storage")
	}
}
