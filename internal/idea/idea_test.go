package idea_test

import (
	"context"
	"testing"
	"time"

	"github.com/ivansuivansu/exchange-controller/internal/domain"
	"github.com/ivansuivansu/exchange-controller/internal/idea"
)

func TestRecoveryBuilderPreservesStructuredContext(t *testing.T) {
	contextData := &domain.RecoverySignal{
		RecentHigh: domain.MustDecimal("110"), LocalLow: domain.MustDecimal("90"),
		RecoveryPrice: domain.MustDecimal("100"), DrawdownPercent: domain.MustDecimal("0.18"),
		RecoveryPercent: domain.MustDecimal("0.11"),
	}
	source := domain.Signal{ID: "signal-1", Market: domain.Market{Base: "A", Quote: "B", Instrument: "A_B"}, ObservedAt: time.Now(), Price: contextData.RecoveryPrice, Recovery: contextData}
	tradeIdea, err := (idea.RecoveryBuilder{}).Build(context.Background(), source)
	if err != nil {
		t.Fatal(err)
	}
	if tradeIdea.Recovery == nil || !tradeIdea.Recovery.RecentHigh.Equal(contextData.RecentHigh) {
		t.Fatal("structured recovery context was not preserved")
	}
	tradeIdea.Recovery.RecentHigh = domain.MustDecimal("999")
	if source.Recovery.RecentHigh.String() != "110" {
		t.Fatal("TradeIdea aliases mutable source context")
	}
}
