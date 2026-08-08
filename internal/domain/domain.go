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
}

type TradeIdea struct {
	ID          string
	SignalID    string
	Market      Market
	CreatedAt   time.Time
	Description string
}

type TradePlanStatus string

const (
	TradePlanPending  TradePlanStatus = "pending"
	TradePlanApproved TradePlanStatus = "approved"
	TradePlanRejected TradePlanStatus = "rejected"
	TradePlanExpired  TradePlanStatus = "expired"
)

type TradePlanParams struct {
	ID             string
	Version        uint64
	IdeaID         string
	Market         Market
	EntryPrice     Decimal
	Quantity       Decimal
	TakeProfit     Decimal
	StopLoss       Decimal
	ApproveBy      time.Time
	EntryExpiresAt time.Time
	Status         TradePlanStatus
}

// TradePlan has no exported fields so approved intent cannot be mutated in
// place. A material edit is expressed as a new version.
type TradePlan struct{ params TradePlanParams }

func NewTradePlan(params TradePlanParams) (TradePlan, error) {
	if params.ID == "" || params.Version == 0 || params.IdeaID == "" {
		return TradePlan{}, errors.New("plan ID, version, and idea ID are required")
	}
	if !params.EntryPrice.IsPositive() || !params.Quantity.IsPositive() ||
		!params.TakeProfit.IsPositive() || !params.StopLoss.IsPositive() {
		return TradePlan{}, errors.New("entry, quantity, take profit, and stop loss must be positive")
	}
	if params.Status == "" {
		params.Status = TradePlanPending
	}
	return TradePlan{params: params}, nil
}

func (p TradePlan) ID() string                         { return p.params.ID }
func (p TradePlan) Version() uint64                    { return p.params.Version }
func (p TradePlan) IdeaID() string                     { return p.params.IdeaID }
func (p TradePlan) Market() Market                     { return p.params.Market }
func (p TradePlan) EntryPrice() Decimal                { return p.params.EntryPrice }
func (p TradePlan) Quantity() Decimal                  { return p.params.Quantity }
func (p TradePlan) TakeProfit() Decimal                { return p.params.TakeProfit }
func (p TradePlan) StopLoss() Decimal                  { return p.params.StopLoss }
func (p TradePlan) ApproveBy() time.Time               { return p.params.ApproveBy }
func (p TradePlan) EntryExpiresAt() time.Time          { return p.params.EntryExpiresAt }
func (p TradePlan) Status() TradePlanStatus            { return p.params.Status }
func (p TradePlan) IsVersion(id string, v uint64) bool { return p.ID() == id && p.Version() == v }

func (p TradePlan) NewVersion(changes TradePlanParams) (TradePlan, error) {
	changes.ID = p.ID()
	changes.Version = p.Version() + 1
	changes.IdeaID = p.IdeaID()
	changes.Market = p.Market()
	changes.Status = TradePlanPending
	return NewTradePlan(changes)
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
)

type PositionState string

const (
	PositionNone   PositionState = "none"
	PositionOpen   PositionState = "open"
	PositionClosed PositionState = "closed"
)

type ExecutionState struct {
	PlanID             string
	PlanVersion        uint64
	RequestedQuantity  Decimal
	FilledQuantity     Decimal
	ProtectedQuantity  Decimal
	AverageEntryPrice  Decimal
	EntryStatus        EntryExecutionState
	PositionStatus     PositionState
	UnknownOrAmbiguous bool
}

func (s ExecutionState) QuantitiesValid() bool {
	return !s.RequestedQuantity.Less(s.FilledQuantity) &&
		!s.FilledQuantity.Less(s.ProtectedQuantity)
}

func (s ExecutionState) HasProtectionRisk() bool {
	return s.ProtectedQuantity.Less(s.FilledQuantity)
}

func (s ExecutionState) LifecycleActive() bool {
	return s.EntryStatus == EntryOpen || s.EntryStatus == EntryPartiallyFilled ||
		s.PositionStatus == PositionOpen || s.UnknownOrAmbiguous
}
