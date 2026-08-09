package backtest

import (
	"time"

	"github.com/ivansuivansu/exchange-controller/internal/domain"
	"github.com/ivansuivansu/exchange-controller/internal/planner"
	"github.com/ivansuivansu/exchange-controller/internal/signal"
)

const PlaceholderPresetName = "PLACEHOLDER_BTC_USD_M1_BASELINE"

type ResearchConfiguration struct {
	PresetName string
	Timeframe  string
	Signal     signal.DrawdownRecoveryConfig
	Planner    planner.V1Config
	Simulation SimulationConfig
}

// PlaceholderBTCUSDResearchPreset is an understandable baseline only. It is
// deliberately not described as optimized or suitable for live trading.
func PlaceholderBTCUSDResearchPreset(feeRate, slippageRate domain.Decimal) ResearchConfiguration {
	market := domain.Market{Base: "BTC", Quote: "USD", Instrument: "BTC_USD"}
	return ResearchConfiguration{
		PresetName: PlaceholderPresetName,
		Timeframe:  "M1",
		Signal: signal.DrawdownRecoveryConfig{
			WindowSize: 60, DrawdownThreshold: domain.MustDecimal("0.01"),
			RecoveryThreshold: domain.MustDecimal("0.005"), Cooldown: 15 * time.Minute,
		},
		Planner: planner.V1Config{
			Name: "placeholder-research-v1", EntryMode: planner.EntryRecoveryClose,
			EntryOffset: domain.Decimal{}, TakeProfitMode: planner.TakeProfitPreviousHigh,
			RetracementTarget: domain.MustDecimal("0.5"), StopLossOffset: domain.MustDecimal("0.005"),
			MinimumRiskReward: domain.MustDecimal("1"), ApprovalTTL: 5 * time.Minute,
			EntryTTL: 25 * time.Minute, FixedQuoteReserve: domain.Decimal{},
		},
		Simulation: SimulationConfig{
			Market: market, Timeframe: time.Minute, StartingCapital: domain.MustDecimal("10000"),
			FeeRate: feeRate, SlippageRate: slippageRate, AmbiguityPolicy: Conservative,
			EntryOrderType: EntryLimitBuy,
		},
	}
}
