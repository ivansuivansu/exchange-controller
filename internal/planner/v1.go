package planner

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ivansuivansu/exchange-controller/internal/domain"
)

type EntryMode string
type TakeProfitMode string

const (
	EntryRecoveryClose      EntryMode      = "RECOVERY_CLOSE"
	EntryOffsetFromRecovery EntryMode      = "OFFSET_FROM_RECOVERY"
	TakeProfitPreviousHigh  TakeProfitMode = "PREVIOUS_HIGH"
	TakeProfitRetracement   TakeProfitMode = "RETRACEMENT"
)

type V1Config struct {
	Name              string
	EntryMode         EntryMode
	EntryOffset       domain.Decimal
	TakeProfitMode    TakeProfitMode
	RetracementTarget domain.Decimal
	StopLossOffset    domain.Decimal
	MinimumRiskReward domain.Decimal
	ApprovalTTL       time.Duration
	EntryTTL          time.Duration
	FixedQuoteReserve domain.Decimal
}

type RejectionReason string

const (
	RejectMalformedSignal     RejectionReason = "malformed_signal"
	RejectInsufficientCapital RejectionReason = "insufficient_capital"
	RejectInvalidEntry        RejectionReason = "invalid_entry"
	RejectInvalidTakeProfit   RejectionReason = "invalid_take_profit"
	RejectInvalidStopLoss     RejectionReason = "invalid_stop_loss"
	RejectInvalidQuantity     RejectionReason = "invalid_quantity"
	RejectInvalidRiskReward   RejectionReason = "invalid_risk_reward"
	RejectMinimumRiskReward   RejectionReason = "minimum_risk_reward"
	RejectInvalidConfig       RejectionReason = "invalid_config"
)

type PlanRejectionError struct {
	Reason RejectionReason
	Detail string
}

func (e *PlanRejectionError) Error() string {
	return fmt.Sprintf("plan rejected (%s): %s", e.Reason, e.Detail)
}

type PlannerV1 struct{ Config V1Config }

var _ TradePlanner = PlannerV1{}

func (p PlannerV1) Plan(ctx context.Context, idea domain.TradeIdea, capital domain.CapitalSnapshot) ([]domain.TradePlan, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	c := p.Config
	if c.Name == "" || c.ApprovalTTL <= 0 || c.EntryTTL <= 0 || c.FixedQuoteReserve.Less(domain.Decimal{}) ||
		!c.StopLossOffset.IsPositive() || !c.MinimumRiskReward.IsPositive() {
		return nil, reject(RejectInvalidConfig, "name, TTLs, reserve, SL offset, and minimum R/R are invalid")
	}
	if idea.Recovery == nil || idea.Market.Base == "" || idea.Market.Quote == "" || idea.Market.Instrument == "" {
		return nil, reject(RejectMalformedSignal, "recovery context and market are required")
	}
	r := *idea.Recovery
	if !r.RecentHigh.IsPositive() || !r.LocalLow.IsPositive() || !r.RecoveryPrice.IsPositive() ||
		!r.LocalLow.Less(r.RecentHigh) || capital.QuoteAsset != idea.Market.Quote || !capital.AvailableQuote.IsPositive() {
		return nil, reject(RejectMalformedSignal, "invalid recovery prices or capital quote asset")
	}
	entry, err := p.entry(r)
	if err != nil || !entry.IsPositive() {
		return nil, reject(RejectInvalidEntry, "entry must be positive")
	}
	tp, err := p.takeProfit(r)
	if err != nil || !entry.Less(tp) {
		return nil, reject(RejectInvalidTakeProfit, "take profit must exceed entry")
	}
	slMultiplier, err := domain.MustDecimal("1").Sub(c.StopLossOffset)
	if err != nil {
		return nil, reject(RejectInvalidConfig, err.Error())
	}
	sl, err := r.LocalLow.Mul(slMultiplier, domain.RoundTowardZero)
	if err != nil || !sl.IsPositive() || !sl.Less(entry) {
		return nil, reject(RejectInvalidStopLoss, "stop loss must be positive and below entry")
	}
	allocated, err := capital.AvailableQuote.Sub(c.FixedQuoteReserve)
	if err != nil || !allocated.IsPositive() {
		return nil, reject(RejectInsufficientCapital, "reserve leaves no quote capital")
	}
	quantity, err := allocated.Div(entry, domain.RoundTowardZero)
	if err != nil || !quantity.IsPositive() {
		return nil, reject(RejectInvalidQuantity, "rounded base quantity is zero")
	}
	notional, err := quantity.Mul(entry, domain.RoundTowardZero)
	if err != nil || capital.AvailableQuote.Less(notional) {
		return nil, reject(RejectInsufficientCapital, "planned cost exceeds available quote")
	}
	upside, _ := tp.Sub(entry)
	downside, _ := entry.Sub(sl)
	if !upside.IsPositive() || !downside.IsPositive() {
		return nil, reject(RejectInvalidRiskReward, "upside and downside must be positive")
	}
	riskReward, err := upside.Div(downside, domain.RoundTowardZero)
	if err != nil || !riskReward.IsPositive() {
		return nil, reject(RejectInvalidRiskReward, "risk/reward is undefined")
	}
	if riskReward.Less(c.MinimumRiskReward) {
		return nil, reject(RejectMinimumRiskReward, "risk/reward is below configured minimum")
	}
	grossUpside, err := upside.Div(entry, domain.RoundTowardZero)
	if err != nil {
		return nil, err
	}
	downsidePercent, err := downside.Div(entry, domain.RoundTowardZero)
	if err != nil {
		return nil, err
	}
	plan, err := domain.NewTradePlan(domain.TradePlanParams{
		ID: fmt.Sprintf("plan-%d", idea.CreatedAt.UnixNano()), Version: 1, IdeaID: idea.ID, Market: idea.Market,
		EntryPrice: entry, Quantity: quantity, QuoteNotional: notional, TakeProfit: tp, StopLoss: sl,
		ApproveBy: idea.CreatedAt.Add(c.ApprovalTTL), EntryTTL: c.EntryTTL,
		RiskReward: riskReward, GrossUpsidePercent: grossUpside, DownsidePercent: downsidePercent, PlannerName: c.Name,
	})
	if err != nil {
		return nil, err
	}
	return []domain.TradePlan{plan}, nil
}

func (p PlannerV1) entry(r domain.RecoverySignal) (domain.Decimal, error) {
	switch p.Config.EntryMode {
	case EntryRecoveryClose:
		return r.RecoveryPrice, nil
	case EntryOffsetFromRecovery:
		multiplier, err := domain.MustDecimal("1").Add(p.Config.EntryOffset)
		if err != nil {
			return domain.Decimal{}, err
		}
		return r.RecoveryPrice.Mul(multiplier, domain.RoundTowardZero)
	default:
		return domain.Decimal{}, errors.New("unsupported entry mode")
	}
}

func (p PlannerV1) takeProfit(r domain.RecoverySignal) (domain.Decimal, error) {
	switch p.Config.TakeProfitMode {
	case TakeProfitPreviousHigh:
		return r.RecentHigh, nil
	case TakeProfitRetracement:
		if !p.Config.RetracementTarget.IsPositive() || domain.MustDecimal("1").Less(p.Config.RetracementTarget) {
			return domain.Decimal{}, errors.New("retracement target must be in (0, 1]")
		}
		move, err := r.RecentHigh.Sub(r.LocalLow)
		if err != nil {
			return domain.Decimal{}, err
		}
		portion, err := move.Mul(p.Config.RetracementTarget, domain.RoundTowardZero)
		if err != nil {
			return domain.Decimal{}, err
		}
		return r.LocalLow.Add(portion)
	default:
		return domain.Decimal{}, errors.New("unsupported take-profit mode")
	}
}

func reject(reason RejectionReason, detail string) error {
	return &PlanRejectionError{Reason: reason, Detail: detail}
}

func IsRejection(err error, reason RejectionReason) bool {
	var rejection *PlanRejectionError
	return errors.As(err, &rejection) && rejection.Reason == reason
}
