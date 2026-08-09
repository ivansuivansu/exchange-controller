package backtest

import (
	"time"

	"github.com/ivansuivansu/exchange-controller/internal/domain"
)

type AmbiguityPolicy string
type EntryOrderType string

const (
	Conservative AmbiguityPolicy = "CONSERVATIVE"
	Optimistic   AmbiguityPolicy = "OPTIMISTIC"
)

const EntryLimitBuy EntryOrderType = "LIMIT_BUY"

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
	EntryOrderType  EntryOrderType
}

type BacktestTradeResult struct {
	PlanID            string
	IdeaID            string
	Instrument        domain.Instrument
	SignalTime        time.Time
	RecentHigh        domain.Decimal
	LocalLow          domain.Decimal
	RecoveryPrice     domain.Decimal
	EntrySubmittedAt  time.Time
	EntryFilledAt     time.Time
	EntryPricePlanned domain.Decimal
	EntryPriceActual  domain.Decimal
	TakeProfit        domain.Decimal
	StopLoss          domain.Decimal
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

type DatasetIdentity struct {
	FirstCandle time.Time
	LastCandle  time.Time
	CandleCount int
	Timeframe   string
	Instrument  domain.Instrument
}

type MonthlyResult struct {
	Month         string
	Trades        int
	Wins          int
	Losses        int
	NetPnL        domain.Decimal
	ReturnPercent domain.Decimal
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
