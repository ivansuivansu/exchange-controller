package backtest

import (
	"testing"
	"time"

	"github.com/ivansuivansu/exchange-controller/internal/domain"
)

func TestReportMaximumDrawdownAndProfitFactor(t *testing.T) {
	r := Report{StartingCapital: domain.MustDecimal("100"), PlannerRejectionsByReason: map[string]int{}}
	for _, trade := range []BacktestTradeResult{
		{ExitReason: ExitTakeProfit, ExitAt: time.Unix(1, 0), NetPnL: domain.MustDecimal("20"), ReturnPercent: domain.MustDecimal("0.2"), CapitalAfter: domain.MustDecimal("120")},
		{ExitReason: ExitStopLoss, ExitAt: time.Unix(2, 0), NetPnL: domain.MustDecimal("-30"), ReturnPercent: domain.MustDecimal("-0.25"), CapitalAfter: domain.MustDecimal("90")},
		{ExitReason: ExitTakeProfit, ExitAt: time.Unix(3, 0), NetPnL: domain.MustDecimal("10"), ReturnPercent: domain.MustDecimal("0.11111111"), CapitalAfter: domain.MustDecimal("100")},
	} {
		r.addResult(trade)
	}
	r.finish(domain.MustDecimal("100"))
	if r.MaximumDrawdown.String() != "0.25" {
		t.Fatalf("drawdown=%s", r.MaximumDrawdown)
	}
	if r.ProfitFactor.String() != "1" {
		t.Fatalf("profit factor=%s", r.ProfitFactor)
	}
	if r.WinningTrades != 2 || r.LosingTrades != 1 || r.WinRate.String() != "0.66666666" {
		t.Fatalf("metrics=%+v", r)
	}
}
