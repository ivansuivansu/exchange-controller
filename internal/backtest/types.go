package backtest

import (
	"time"

	"github.com/ivansuivansu/exchange-controller/internal/domain"
)

type AmbiguityPolicy string

const (
	Conservative AmbiguityPolicy = "CONSERVATIVE"
	Optimistic   AmbiguityPolicy = "OPTIMISTIC"
)

type ExitReason string

const (
	ExitTakeProfit ExitReason = "TP"
	ExitStopLoss   ExitReason = "SL"
	ExitExpiration ExitReason = "EXPIRATION"
	ExitOther      ExitReason = "OTHER"
)

type SimulationConfig struct {
	Market          domain.Market
	Timeframe       time.Duration
	StartingCapital domain.Decimal
	FeeRate         domain.Decimal
	SlippageRate    domain.Decimal
	AmbiguityPolicy AmbiguityPolicy
}

type BacktestTradeResult struct {
	PlanID            string
	IdeaID            string
	Instrument        domain.Instrument
	SignalTime        time.Time
	EntrySubmittedAt  time.Time
	EntryFilledAt     time.Time
	EntryPricePlanned domain.Decimal
	EntryPriceActual  domain.Decimal
	ExitAt            time.Time
	ExitReason        ExitReason
	ExitPrice         domain.Decimal
	Quantity          domain.Decimal
	EntryFee          domain.Decimal
	ExitFee           domain.Decimal
	GrossPnL          domain.Decimal
	NetPnL            domain.Decimal
	ReturnPercent     domain.Decimal
	CapitalBefore     domain.Decimal
	CapitalAfter      domain.Decimal
	WasAmbiguous      bool
}

type EquityPoint struct {
	At      time.Time
	Capital domain.Decimal
}

type Report struct {
	StartingCapital           domain.Decimal
	EndingCapital             domain.Decimal
	NetProfitLoss             domain.Decimal
	TotalReturnPercent        domain.Decimal
	TotalSignals              int
	PlansProduced             int
	PlannerRejections         int
	PlannerRejectionsByReason map[string]int
	EntriesFilled             int
	EntriesExpired            int
	WinningTrades             int
	LosingTrades              int
	WinRate                   domain.Decimal
	AverageWin                domain.Decimal
	AverageLoss               domain.Decimal
	AverageTradeReturn        domain.Decimal
	LargestWin                domain.Decimal
	LargestLoss               domain.Decimal
	ProfitFactor              domain.Decimal
	MaximumDrawdown           domain.Decimal
	AmbiguousTradeCount       int
	TotalFeesPaid             domain.Decimal
	Trades                    []BacktestTradeResult
	EquityCurve               []EquityPoint
}
