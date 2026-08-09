package backtest

import (
	"encoding/csv"
	"fmt"
	"io"
	"sort"
	"strconv"
	"time"

	"github.com/ivansuivansu/exchange-controller/internal/domain"
)

func WriteTradeCSV(w io.Writer, trades []BacktestTradeResult) error {
	c := csv.NewWriter(w)
	header := []string{"plan_id", "idea_id", "signal_time", "recent_high", "local_low", "recovery_price", "entry_submitted_at", "entry_filled_at", "planned_entry", "actual_entry", "take_profit", "stop_loss", "exit_at", "exit_reason", "exit_price", "quantity", "entry_fee", "exit_fee", "gross_pnl", "net_pnl", "return_percent", "capital_before", "capital_after", "ambiguous"}
	if err := c.Write(header); err != nil {
		return err
	}
	for _, t := range trades {
		record := []string{t.PlanID, t.IdeaID, formatTime(t.SignalTime), t.RecentHigh.String(), t.LocalLow.String(), t.RecoveryPrice.String(), formatTime(t.EntrySubmittedAt), formatTime(t.EntryFilledAt), t.EntryPricePlanned.String(), t.EntryPriceActual.String(), t.TakeProfit.String(), t.StopLoss.String(), formatTime(t.ExitAt), string(t.ExitReason), t.ExitPrice.String(), t.Quantity.String(), t.EntryFee.String(), t.ExitFee.String(), t.GrossPnL.String(), t.NetPnL.String(), t.ReturnPercent.String(), t.CapitalBefore.String(), t.CapitalAfter.String(), strconv.FormatBool(t.WasAmbiguous)}
		if err := c.Write(record); err != nil {
			return err
		}
	}
	c.Flush()
	return c.Error()
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}

func MonthlyBreakdown(report Report) []MonthlyResult {
	type accumulator struct {
		result        MonthlyResult
		capitalBefore domain.Decimal
	}
	months := make(map[string]*accumulator)
	for _, trade := range report.Trades {
		if trade.ExitReason == ExitExpiration {
			continue
		}
		key := trade.ExitAt.UTC().Format("2006-01")
		item := months[key]
		if item == nil {
			item = &accumulator{result: MonthlyResult{Month: key}, capitalBefore: trade.CapitalBefore}
			months[key] = item
		}
		item.result.Trades++
		item.result.NetPnL, _ = item.result.NetPnL.Add(trade.NetPnL)
		if trade.NetPnL.IsPositive() {
			item.result.Wins++
		} else if trade.NetPnL.Less(domain.Decimal{}) {
			item.result.Losses++
		}
	}
	keys := make([]string, 0, len(months))
	for key := range months {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]MonthlyResult, 0, len(keys))
	for _, key := range keys {
		item := months[key]
		if item.capitalBefore.IsPositive() {
			item.result.ReturnPercent, _ = item.result.NetPnL.Div(item.capitalBefore, domain.RoundTowardZero)
		}
		result = append(result, item.result)
	}
	return result
}

func WriteMonthlyCSV(w io.Writer, months []MonthlyResult) error {
	c := csv.NewWriter(w)
	if err := c.Write([]string{"month", "trades", "wins", "losses", "net_pnl", "return_percent"}); err != nil {
		return err
	}
	for _, m := range months {
		if err := c.Write([]string{m.Month, strconv.Itoa(m.Trades), strconv.Itoa(m.Wins), strconv.Itoa(m.Losses), m.NetPnL.String(), m.ReturnPercent.String()}); err != nil {
			return err
		}
	}
	c.Flush()
	return c.Error()
}

func WriteResearchReport(w io.Writer, config ResearchConfiguration, dataset DatasetIdentity, report Report) error {
	p := func(format string, args ...any) error { _, err := fmt.Fprintf(w, format, args...); return err }
	if err := p("BACKTEST RESEARCH — %s (PLACEHOLDER; NOT OPTIMIZED)\n", config.PresetName); err != nil {
		return err
	}
	if err := p("Configuration:\n  timeframe: %s\n  lookback: %d\n  drawdown_threshold: %s\n  recovery_threshold: %s\n  cooldown_reset: %s\n  entry_mode: %s\n  entry_offset: %s\n  tp_mode: %s\n  retracement_target: %s\n  sl_offset: %s\n  minimum_risk_reward: %s\n  entry_ttl: %s\n  technical_reserve: %s\n  fee_rate: %s\n  slippage_rate: %s\n  ambiguity_policy: %s\n",
		config.Timeframe, config.Signal.WindowSize, config.Signal.DrawdownThreshold, config.Signal.RecoveryThreshold, config.Signal.Cooldown,
		config.Planner.EntryMode, config.Planner.EntryOffset, config.Planner.TakeProfitMode, config.Planner.RetracementTarget,
		config.Planner.StopLossOffset, config.Planner.MinimumRiskReward, config.Planner.EntryTTL, config.Planner.FixedQuoteReserve,
		config.Simulation.FeeRate, config.Simulation.SlippageRate, config.Simulation.AmbiguityPolicy); err != nil {
		return err
	}
	if err := p("Dataset:\n  instrument: %s\n  timeframe: %s\n  first_candle: %s\n  last_candle: %s\n  candle_count: %d\n", dataset.Instrument, dataset.Timeframe, formatTime(dataset.FirstCandle), formatTime(dataset.LastCandle), dataset.CandleCount); err != nil {
		return err
	}
	if err := p("Summary:\n  starting_capital: %s\n  ending_capital: %s\n  net_pnl: %s\n  total_return: %s\n  signals: %d\n  plans: %d\n  planner_rejections: %d\n  filled_entries: %d\n  expired_entries: %d\n  wins: %d\n  losses: %d\n  win_rate: %s\n  average_win: %s\n  average_loss: %s\n  profit_factor: %s\n  maximum_drawdown: %s\n  total_fees: %s\n  ambiguous_trades: %d\n",
		report.StartingCapital, report.EndingCapital, report.NetProfitLoss, report.TotalReturnPercent, report.TotalSignals,
		report.PlansProduced, report.PlannerRejections, report.EntriesFilled, report.EntriesExpired, report.WinningTrades,
		report.LosingTrades, report.WinRate, report.AverageWin, report.AverageLoss, report.ProfitFactor,
		report.MaximumDrawdown, report.TotalFeesPaid, report.AmbiguousTradeCount); err != nil {
		return err
	}
	if err := p("Planner rejections by reason:\n"); err != nil {
		return err
	}
	keys := make([]string, 0, len(report.PlannerRejectionsByReason))
	for key := range report.PlannerRejectionsByReason {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if err := p("  %s: %d\n", key, report.PlannerRejectionsByReason[key]); err != nil {
			return err
		}
	}
	if err := p("Monthly breakdown:\n  month,trades,wins,losses,net_pnl,return_percent\n"); err != nil {
		return err
	}
	for _, m := range MonthlyBreakdown(report) {
		if err := p("  %s,%d,%d,%d,%s,%s\n", m.Month, m.Trades, m.Wins, m.Losses, m.NetPnL, m.ReturnPercent); err != nil {
			return err
		}
	}
	return nil
}
