package backtest

import "github.com/ivansuivansu/exchange-controller/internal/domain"

func (r *Report) addResult(result BacktestTradeResult) {
	r.Trades = append(r.Trades, result)
	if result.ExitReason == ExitExpiration {
		return
	}
	r.EquityCurve = append(r.EquityCurve, EquityPoint{At: result.ExitAt, Capital: result.CapitalAfter})
	fees, _ := result.EntryFee.Add(result.ExitFee)
	r.TotalFeesPaid, _ = r.TotalFeesPaid.Add(fees)
	if result.WasAmbiguous {
		r.AmbiguousTradeCount++
	}
	zero := domain.Decimal{}
	if zero.Less(result.NetPnL) {
		r.WinningTrades++
		if r.LargestWin.Less(result.NetPnL) {
			r.LargestWin = result.NetPnL
		}
	} else if result.NetPnL.Less(zero) {
		r.LosingTrades++
		if r.LargestLoss.IsZero() || result.NetPnL.Less(r.LargestLoss) {
			r.LargestLoss = result.NetPnL
		}
	}
}

func (r *Report) finish(ending domain.Decimal) {
	r.EndingCapital = ending
	r.NetProfitLoss, _ = ending.Sub(r.StartingCapital)
	if r.StartingCapital.IsPositive() {
		r.TotalReturnPercent, _ = r.NetProfitLoss.Div(r.StartingCapital, domain.RoundTowardZero)
	}
	var wins, lossesAbs, returns domain.Decimal
	var tradeCount int
	peak := r.StartingCapital
	for _, trade := range r.Trades {
		if trade.ExitReason == ExitExpiration {
			continue
		}
		tradeCount++
		returns, _ = returns.Add(trade.ReturnPercent)
		if trade.NetPnL.IsPositive() {
			wins, _ = wins.Add(trade.NetPnL)
		}
		if trade.NetPnL.Less(domain.Decimal{}) {
			lossAbs, _ := domain.Decimal{}.Sub(trade.NetPnL)
			lossesAbs, _ = lossesAbs.Add(lossAbs)
		}
		if peak.Less(trade.CapitalAfter) {
			peak = trade.CapitalAfter
		}
		if trade.CapitalAfter.Less(peak) {
			drop, _ := peak.Sub(trade.CapitalAfter)
			drawdown, _ := drop.Div(peak, domain.RoundTowardZero)
			if r.MaximumDrawdown.Less(drawdown) {
				r.MaximumDrawdown = drawdown
			}
		}
	}
	if r.WinningTrades > 0 {
		r.AverageWin, _ = wins.Div(domain.MustDecimal(intString(r.WinningTrades)), domain.RoundTowardZero)
	}
	if r.LosingTrades > 0 {
		averageAbs, _ := lossesAbs.Div(domain.MustDecimal(intString(r.LosingTrades)), domain.RoundTowardZero)
		r.AverageLoss, _ = domain.Decimal{}.Sub(averageAbs)
	}
	if tradeCount > 0 {
		r.WinRate, _ = domain.MustDecimal(intString(r.WinningTrades)).Div(domain.MustDecimal(intString(tradeCount)), domain.RoundTowardZero)
		r.AverageTradeReturn, _ = returns.Div(domain.MustDecimal(intString(tradeCount)), domain.RoundTowardZero)
	}
	if lossesAbs.IsPositive() {
		r.ProfitFactor, _ = wins.Div(lossesAbs, domain.RoundTowardZero)
	}
}

func intString(value int) string {
	if value == 0 {
		return "0"
	}
	var digits [20]byte
	i := len(digits)
	for value > 0 {
		i--
		digits[i] = byte('0' + value%10)
		value /= 10
	}
	return string(digits[i:])
}
