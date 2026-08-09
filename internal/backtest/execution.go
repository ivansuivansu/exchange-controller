package backtest

import (
	"errors"
	"fmt"
	"time"

	"github.com/ivansuivansu/exchange-controller/internal/domain"
	"github.com/ivansuivansu/exchange-controller/internal/execution"
)

type pendingEntry struct {
	plan          domain.TradePlan
	signalTime    time.Time
	recovery      domain.RecoverySignal
	submittedAt   time.Time
	expiresAt     time.Time
	capitalBefore domain.Decimal
}

type openPosition struct {
	pendingEntry
	filledAt       time.Time
	fillCandleOpen time.Time
	actualEntry    domain.Decimal
	entryCost      domain.Decimal
	entryFee       domain.Decimal
}

type BacktestExecutionEngine struct {
	config         SimulationConfig
	capital        domain.Decimal
	pending        *pendingEntry
	position       *openPosition
	entriesFilled  int
	entriesExpired int
}

var ErrBacktestActiveLifecycle = errors.New("backtest lifecycle already active")

func NewBacktestExecutionEngine(config SimulationConfig) (*BacktestExecutionEngine, error) {
	one := domain.MustDecimal("1")
	if config.Market.Base == "" || config.Market.Quote == "" || config.Market.Instrument == "" || config.Timeframe <= 0 ||
		!config.StartingCapital.IsPositive() || config.FeeRate.Less(domain.Decimal{}) || !config.FeeRate.Less(one) ||
		config.SlippageRate.Less(domain.Decimal{}) || !config.SlippageRate.Less(one) {
		return nil, errors.New("invalid backtest capital, fee, or slippage")
	}
	if config.AmbiguityPolicy == "" {
		config.AmbiguityPolicy = Conservative
	}
	if config.EntryOrderType == "" {
		config.EntryOrderType = EntryLimitBuy
	}
	if config.AmbiguityPolicy != Conservative && config.AmbiguityPolicy != Optimistic {
		return nil, errors.New("invalid ambiguity policy")
	}
	if config.EntryOrderType != EntryLimitBuy {
		return nil, errors.New("unsupported backtest entry order type")
	}
	return &BacktestExecutionEngine{config: config, capital: config.StartingCapital}, nil
}

func (e *BacktestExecutionEngine) Capital() domain.Decimal { return e.capital }
func (e *BacktestExecutionEngine) Active() bool            { return e.pending != nil || e.position != nil }

// PlanningCapital reserves enough quote currency for the configured entry
// fee, so a planner that allocates all available quote does not create an
// impossible simulated order when fees are non-zero.
func (e *BacktestExecutionEngine) PlanningCapital() (domain.Decimal, error) {
	multiplier, err := domain.MustDecimal("1").Add(e.config.FeeRate)
	if err != nil {
		return domain.Decimal{}, err
	}
	available, err := e.capital.Div(multiplier, domain.RoundTowardZero)
	if err != nil || e.config.FeeRate.IsZero() {
		return available, err
	}
	// Entry fees round away from zero; retain one Decimal unit to cover the
	// maximum difference introduced by that rounding.
	return available.Sub(domain.DecimalFromUnits(1))
}

func (e *BacktestExecutionEngine) Submit(plan domain.TradePlan, approval domain.Approval, signalTime, submittedAt time.Time, recovery ...domain.RecoverySignal) error {
	if e.Active() {
		return ErrBacktestActiveLifecycle
	}
	if approval.Decision != domain.ApprovalApproved || !approval.AppliesTo(plan) {
		return execution.ErrPlanNotApproved
	}
	var context domain.RecoverySignal
	if len(recovery) > 0 {
		context = recovery[0]
	}
	e.pending = &pendingEntry{plan: plan, signalTime: signalTime, recovery: context, submittedAt: submittedAt,
		expiresAt: plan.EntryExpiresAt(submittedAt), capitalBefore: e.capital}
	return nil
}

func (e *BacktestExecutionEngine) OnCandle(candle domain.Candle) (*BacktestTradeResult, error) {
	if e.pending != nil {
		return e.processPending(candle)
	}
	if e.position != nil {
		return e.processPosition(candle)
	}
	return nil, nil
}

func (e *BacktestExecutionEngine) processPending(candle domain.Candle) (*BacktestTradeResult, error) {
	pending := e.pending
	if candle.OpenTime.Before(pending.submittedAt) {
		return nil, nil
	}
	if candle.CloseTime.After(pending.expiresAt) {
		result := &BacktestTradeResult{PlanID: pending.plan.ID(), IdeaID: pending.plan.IdeaID(),
			Instrument: pending.plan.Market().Instrument, SignalTime: pending.signalTime,
			RecentHigh: pending.recovery.RecentHigh, LocalLow: pending.recovery.LocalLow, RecoveryPrice: pending.recovery.RecoveryPrice,
			EntrySubmittedAt: pending.submittedAt, EntryPricePlanned: pending.plan.EntryPrice(),
			TakeProfit: pending.plan.TakeProfit(), StopLoss: pending.plan.StopLoss(),
			ExitAt: pending.expiresAt, ExitReason: ExitExpiration,
			Quantity: pending.plan.Quantity(), CapitalBefore: pending.capitalBefore, CapitalAfter: e.capital}
		e.pending = nil
		e.entriesExpired++
		return result, nil
	}
	if pending.plan.EntryPrice().Less(candle.Low) || candle.High.Less(pending.plan.EntryPrice()) {
		return nil, nil
	}
	actual, err := e.entryFillPrice(pending.plan)
	if err != nil {
		return nil, err
	}
	cost, err := pending.plan.Quantity().Mul(actual, domain.RoundAwayFromZero)
	if err != nil {
		return nil, err
	}
	fee, err := cost.Mul(e.config.FeeRate, domain.RoundAwayFromZero)
	if err != nil {
		return nil, err
	}
	total, err := cost.Add(fee)
	if err != nil {
		return nil, err
	}
	if e.capital.Less(total) {
		return nil, fmt.Errorf("entry cost plus fee exceeds simulated capital")
	}
	e.position = &openPosition{pendingEntry: *pending, filledAt: candle.CloseTime,
		fillCandleOpen: candle.OpenTime, actualEntry: actual, entryCost: cost, entryFee: fee}
	e.pending = nil
	e.entriesFilled++
	return nil, nil
}

func (e *BacktestExecutionEngine) entryFillPrice(plan domain.TradePlan) (domain.Decimal, error) {
	switch e.config.EntryOrderType {
	case EntryLimitBuy:
		// A BUY limit can fill at its limit or better, never above it. The OHLC
		// MVP has no information to award price improvement, so it fills exactly
		// at the approved limit. A future MARKET entry model belongs in another
		// branch and may apply adverse BUY slippage there.
		return plan.EntryPrice(), nil
	default:
		return domain.Decimal{}, errors.New("unsupported backtest entry order type")
	}
}

func (e *BacktestExecutionEngine) processPosition(candle domain.Candle) (*BacktestTradeResult, error) {
	position := e.position
	if !candle.OpenTime.After(position.fillCandleOpen) {
		return nil, nil
	}
	tpHit := !candle.High.Less(position.plan.TakeProfit())
	slHit := !position.plan.StopLoss().Less(candle.Low)
	if !tpHit && !slHit {
		return nil, nil
	}
	ambiguous := tpHit && slHit
	reason, plannedExit := ExitTakeProfit, position.plan.TakeProfit()
	if slHit && (!tpHit || e.config.AmbiguityPolicy == Conservative) {
		reason, plannedExit = ExitStopLoss, position.plan.StopLoss()
	}
	multiplier, err := domain.MustDecimal("1").Sub(e.config.SlippageRate)
	if err != nil {
		return nil, err
	}
	actualExit, err := plannedExit.Mul(multiplier, domain.RoundTowardZero)
	if err != nil {
		return nil, err
	}
	exitValue, err := position.plan.Quantity().Mul(actualExit, domain.RoundTowardZero)
	if err != nil {
		return nil, err
	}
	exitFee, err := exitValue.Mul(e.config.FeeRate, domain.RoundAwayFromZero)
	if err != nil {
		return nil, err
	}
	gross, err := exitValue.Sub(position.entryCost)
	if err != nil {
		return nil, err
	}
	fees, err := position.entryFee.Add(exitFee)
	if err != nil {
		return nil, err
	}
	net, err := gross.Sub(fees)
	if err != nil {
		return nil, err
	}
	after, err := position.capitalBefore.Add(net)
	if err != nil {
		return nil, err
	}
	returnPercent, err := net.Div(position.capitalBefore, domain.RoundTowardZero)
	if err != nil {
		return nil, err
	}
	e.capital, e.position = after, nil
	return &BacktestTradeResult{
		PlanID: position.plan.ID(), IdeaID: position.plan.IdeaID(), Instrument: position.plan.Market().Instrument,
		SignalTime: position.signalTime, EntrySubmittedAt: position.submittedAt, EntryFilledAt: position.filledAt,
		RecentHigh: position.recovery.RecentHigh, LocalLow: position.recovery.LocalLow, RecoveryPrice: position.recovery.RecoveryPrice,
		EntryPricePlanned: position.plan.EntryPrice(), EntryPriceActual: position.actualEntry,
		TakeProfit: position.plan.TakeProfit(), StopLoss: position.plan.StopLoss(),
		ExitAt: candle.CloseTime, ExitReason: reason, ExitPrice: actualExit, Quantity: position.plan.Quantity(),
		EntryFee: position.entryFee, ExitFee: exitFee, GrossPnL: gross, NetPnL: net,
		ReturnPercent: returnPercent, CapitalBefore: position.capitalBefore, CapitalAfter: after,
		WasAmbiguous: ambiguous,
	}, nil
}
