package backtest

import (
	"bytes"
	"encoding/csv"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/ivansuivansu/exchange-controller/internal/domain"
)

func TestTradeCSVExportAndMonthlyAggregation(t *testing.T) {
	at := time.Date(2025, 2, 3, 0, 0, 0, 0, time.UTC)
	report := Report{Trades: []BacktestTradeResult{{PlanID: "p1", IdeaID: "i1", SignalTime: at, ExitAt: at, ExitReason: ExitTakeProfit, NetPnL: domain.MustDecimal("10"), CapitalBefore: domain.MustDecimal("1000"), CapitalAfter: domain.MustDecimal("1010")}, {PlanID: "p2", ExitAt: at.Add(time.Hour), ExitReason: ExitStopLoss, NetPnL: domain.MustDecimal("-5"), CapitalBefore: domain.MustDecimal("1010"), CapitalAfter: domain.MustDecimal("1005")}, {PlanID: "p3", ExitAt: at, ExitReason: ExitExpiration}}}
	var output bytes.Buffer
	if err := WriteTradeCSV(&output, report.Trades); err != nil {
		t.Fatal(err)
	}
	records, err := csv.NewReader(strings.NewReader(output.String())).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 4 || records[0][0] != "plan_id" || records[1][0] != "p1" || records[3][13] != string(ExitExpiration) {
		t.Fatalf("trade CSV=%v", records)
	}
	months := MonthlyBreakdown(report)
	if len(months) != 1 || months[0].Month != "2025-02" || months[0].Trades != 2 || months[0].Wins != 1 || months[0].Losses != 1 || months[0].NetPnL.String() != "5" || months[0].ReturnPercent.String() != "0.005" {
		t.Fatalf("months=%+v", months)
	}
}

func TestReportIdentifiesConfigurationAndIsDeterministic(t *testing.T) {
	config := PlaceholderBTCUSDResearchPreset(domain.MustDecimal("0.001"), domain.Decimal{})
	report := runProductionBacktest(t, historicalSequence(1), "0.5")
	dataset := DatasetIdentity{FirstCandle: historicalSequence(1)[0].OpenTime, LastCandle: historicalSequence(1)[4].OpenTime, CandleCount: 5, Timeframe: "M1", Instrument: "BTC_USD"}
	write := func() string {
		var b bytes.Buffer
		if err := WriteResearchReport(&b, config, dataset, report); err != nil {
			t.Fatal(err)
		}
		return b.String()
	}
	first, second := write(), write()
	if first != second {
		t.Fatal("report output is not deterministic")
	}
	for _, required := range []string{"PLACEHOLDER; NOT OPTIMIZED", "lookback: 60", "drawdown_threshold: 0.01", "fee_rate: 0.001", "instrument: BTC_USD", "Planner rejections by reason", "Monthly breakdown"} {
		if !strings.Contains(first, required) {
			t.Errorf("report missing %q", required)
		}
	}
	if !reflect.DeepEqual(report, runProductionBacktest(t, historicalSequence(1), "0.5")) {
		t.Fatal("repeated backtest changed")
	}
}
