package domain

import (
	"errors"
	"time"
)

type Asset string
type Instrument string

type Market struct {
	Base       Asset
	Quote      Asset
	Instrument Instrument
}

type MarketEvent struct {
	Market Market
	Price  Decimal
	At     time.Time
}

// Candle is an instrument-agnostic OHLCV observation for a fixed interval.
// VolumeAvailable distinguishes unavailable volume from an actual zero.
type Candle struct {
	Market          Market
	Open            Decimal
	High            Decimal
	Low             Decimal
	Close           Decimal
	Volume          Decimal
	VolumeAvailable bool
	OpenTime        time.Time
	CloseTime       time.Time
}

type MarketState struct {
	Market    Market
	LastPrice Decimal
	AsOf      time.Time
}

type Signal struct {
	ID          string
	Market      Market
	ObservedAt  time.Time
	Price       Decimal
	Description string
	Recovery    *RecoverySignal
}

type RecoverySignal struct {
	RecentHigh      Decimal
	LocalLow        Decimal
	RecoveryPrice   Decimal
	DrawdownPercent Decimal
	RecoveryPercent Decimal
}

type TradeIdea struct {
	ID             string
	SignalIDs      []string
	Market         Market
	ReferencePrice Decimal
	CreatedAt      time.Time
	Description    string
	Recovery       *RecoverySignal
}

type CapitalSnapshot struct {
	QuoteAsset     Asset
	AvailableQuote Decimal
	AsOf           time.Time
}

type TradePlanLifecycleStatus string

const (
	PlanPending  TradePlanLifecycleStatus = "pending"
	PlanApproved TradePlanLifecycleStatus = "approved"
	PlanRejected TradePlanLifecycleStatus = "rejected"
	PlanExpired  TradePlanLifecycleStatus = "expired"
)

// TradePlanLifecycle records mutable workflow state separately from immutable
// TradePlan intent.
type TradePlanLifecycle struct {
	PlanID      string
	PlanVersion uint64
	Status      TradePlanLifecycleStatus
	UpdatedAt   time.Time
}

type TradePlanParams struct {
	ID                 string
	Version            uint64
	IdeaID             string
	Market             Market
	EntryPrice         Decimal
	Quantity           Decimal
	TakeProfit         Decimal
	StopLoss           Decimal
	ApproveBy          time.Time
	EntryTTL           time.Duration
	QuoteNotional      Decimal
	RiskReward         Decimal
	GrossUpsidePercent Decimal
	DownsidePercent    Decimal
	PlannerName        string
}

// TradePlan has no exported fields so approved intent cannot be mutated in
// place. A material edit is expressed as a new version.
type TradePlan struct{ params TradePlanParams }

func NewTradePlan(params TradePlanParams) (TradePlan, error) {
	if params.ID == "" || params.Version == 0 || params.IdeaID == "" {
		return TradePlan{}, errors.New("plan ID, version, and idea ID are required")
	}
	if params.Market.Base == "" || params.Market.Quote == "" || params.Market.Instrument == "" {
		return TradePlan{}, errors.New("market base, quote, and instrument are required")
	}
	if !params.EntryPrice.IsPositive() || !params.Quantity.IsPositive() ||
		!params.TakeProfit.IsPositive() || !params.StopLoss.IsPositive() {
		return TradePlan{}, errors.New("entry, quantity, take profit, and stop loss must be positive")
	}
	if params.ApproveBy.IsZero() || params.EntryTTL <= 0 {
		return TradePlan{}, errors.New("approve-by and a positive entry TTL are required")
	}
	if !params.QuoteNotional.IsPositive() || !params.RiskReward.IsPositive() ||
		!params.GrossUpsidePercent.IsPositive() || !params.DownsidePercent.IsPositive() || params.PlannerName == "" {
		return TradePlan{}, errors.New("notional, projections, risk/reward, and planner name are required")
	}
	return TradePlan{params: params}, nil
}

func (p TradePlan) ID() string                  { return p.params.ID }
func (p TradePlan) Version() uint64             { return p.params.Version }
func (p TradePlan) IdeaID() string              { return p.params.IdeaID }
func (p TradePlan) Market() Market              { return p.params.Market }
func (p TradePlan) EntryPrice() Decimal         { return p.params.EntryPrice }
func (p TradePlan) Quantity() Decimal           { return p.params.Quantity }
func (p TradePlan) TakeProfit() Decimal         { return p.params.TakeProfit }
func (p TradePlan) StopLoss() Decimal           { return p.params.StopLoss }
func (p TradePlan) ApproveBy() time.Time        { return p.params.ApproveBy }
func (p TradePlan) EntryTTL() time.Duration     { return p.params.EntryTTL }
func (p TradePlan) QuoteNotional() Decimal      { return p.params.QuoteNotional }
func (p TradePlan) RiskReward() Decimal         { return p.params.RiskReward }
func (p TradePlan) GrossUpsidePercent() Decimal { return p.params.GrossUpsidePercent }
func (p TradePlan) DownsidePercent() Decimal    { return p.params.DownsidePercent }
func (p TradePlan) PlannerName() string         { return p.params.PlannerName }
func (p TradePlan) EntryExpiresAt(submittedAt time.Time) time.Time {
	return submittedAt.Add(p.EntryTTL())
}
func (p TradePlan) IsVersion(id string, v uint64) bool { return p.ID() == id && p.Version() == v }

// TradePlanEdits contains only material fields that may be changed when
// creating a new version. Nil fields retain their value from the prior plan.
type TradePlanEdits struct {
	EntryPrice *Decimal
	Quantity   *Decimal
	TakeProfit *Decimal
	StopLoss   *Decimal
	ApproveBy  *time.Time
	EntryTTL   *time.Duration
}

func (p TradePlan) Edit(edits TradePlanEdits) (TradePlan, error) {
	params := p.params
	params.Version++
	changed := false
	if edits.EntryPrice != nil {
		params.EntryPrice, changed = *edits.EntryPrice, true
	}
	if edits.Quantity != nil {
		params.Quantity, changed = *edits.Quantity, true
	}
	if edits.TakeProfit != nil {
		params.TakeProfit, changed = *edits.TakeProfit, true
	}
	if edits.StopLoss != nil {
		params.StopLoss, changed = *edits.StopLoss, true
	}
	if edits.ApproveBy != nil {
		params.ApproveBy, changed = *edits.ApproveBy, true
	}
	if edits.EntryTTL != nil {
		params.EntryTTL, changed = *edits.EntryTTL, true
	}
	if !changed {
		return TradePlan{}, errors.New("at least one material edit is required")
	}
	return NewTradePlan(params)
}

type ApprovalDecision string

const (
	ApprovalApproved ApprovalDecision = "approved"
	ApprovalRejected ApprovalDecision = "rejected"
)

type Approval struct {
	PlanID      string
	PlanVersion uint64
	Decision    ApprovalDecision
	DecidedAt   time.Time
}

func (a Approval) AppliesTo(plan TradePlan) bool {
	return plan.IsVersion(a.PlanID, a.PlanVersion)
}

type EntryExecutionState string

const (
	EntryNotSubmitted    EntryExecutionState = "not_submitted"
	EntryOpen            EntryExecutionState = "open"
	EntryPartiallyFilled EntryExecutionState = "partially_filled"
	EntryFilled          EntryExecutionState = "filled"
	EntryCancelled       EntryExecutionState = "cancelled"
	EntryExpired         EntryExecutionState = "expired"
	EntryRejected        EntryExecutionState = "rejected"
)

type PositionState string

const (
	PositionNone   PositionState = "none"
	PositionOpen   PositionState = "open"
	PositionClosed PositionState = "closed"
)

type ExecutionKnowledgeState string

const (
	ExecutionKnown     ExecutionKnowledgeState = "known"
	ExecutionUnknown   ExecutionKnowledgeState = "unknown"
	ExecutionAmbiguous ExecutionKnowledgeState = "ambiguous"
)

type ExecutionState struct {
	PlanID            string
	PlanVersion       uint64
	RequestedQuantity Decimal
	FilledQuantity    Decimal
	ProtectedQuantity Decimal
	AverageEntryPrice Decimal
	EntryStatus       EntryExecutionState
	PositionStatus    PositionState
	Knowledge         ExecutionKnowledgeState
}

func (s ExecutionState) QuantitiesValid() bool {
	zero := Decimal{}
	return !s.ProtectedQuantity.Less(zero) &&
		!s.FilledQuantity.Less(zero) &&
		!s.RequestedQuantity.Less(zero) &&
		!s.RequestedQuantity.Less(s.FilledQuantity) &&
		!s.FilledQuantity.Less(s.ProtectedQuantity)
}

func (s ExecutionState) HasProtectionRisk() bool {
	return s.ProtectedQuantity.Less(s.FilledQuantity)
}

func (s ExecutionState) LifecycleActive() bool {
	return s.EntryStatus == EntryOpen || s.EntryStatus == EntryPartiallyFilled ||
		s.PositionStatus == PositionOpen || s.Knowledge != ExecutionKnown
}
