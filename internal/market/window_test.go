package market_test

import (
	"testing"

	"github.com/ivansuivansu/exchange-controller/internal/domain"
	"github.com/ivansuivansu/exchange-controller/internal/market"
)

func TestRollingWindow(t *testing.T) {
	window, err := market.NewRollingWindow(3)
	if err != nil {
		t.Fatal(err)
	}
	for _, price := range []string{"1", "2", "3", "4"} {
		window.Add(domain.MarketEvent{Price: domain.MustDecimal(price)})
	}
	events := window.Events()
	if len(events) != 3 || events[0].Price.String() != "2" || events[2].Price.String() != "4" {
		t.Fatalf("unexpected window: %+v", events)
	}
	events[0].Price = domain.MustDecimal("99")
	if window.Events()[0].Price.String() != "2" {
		t.Fatal("Events exposed internal window storage")
	}
}
