package telegram

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/ivansuivansu/exchange-controller/internal/approval"
	"github.com/ivansuivansu/exchange-controller/internal/domain"
)

var (
	ErrUnauthorized       = errors.New("unauthorized Telegram actor")
	ErrUnknownPlan        = errors.New("unknown trade plan")
	ErrStaleVersion       = errors.New("stale trade plan version")
	ErrExpiredPlan        = errors.New("trade plan approval deadline has expired")
	ErrAlreadyDecided     = errors.New("trade plan has already been decided")
	ErrEditRequired       = errors.New("material edits are required")
	ErrPlanEdited         = errors.New("trade plan was replaced by an edited version")
	ErrUnsupportedCommand = errors.New("unsupported command")
)

type decisionResult struct {
	approval domain.Approval
	err      error
}

type planRecord struct {
	plan    domain.TradePlan
	decided bool
	waiter  chan decisionResult
}

type Approver struct {
	mu           sync.Mutex
	config       Config
	messenger    Messenger
	status       StatusSource
	now          func() time.Time
	allowedUsers map[int64]struct{}
	allowedChats map[int64]struct{}
	records      map[string]*planRecord
	latest       map[string]uint64
	pendingKey   string
}

var _ approval.PlanApprover = (*Approver)(nil)

func NewApprover(config Config, messenger Messenger, status StatusSource) *Approver {
	a := &Approver{
		config: config, messenger: messenger, status: status, now: time.Now,
		allowedUsers: make(map[int64]struct{}), allowedChats: make(map[int64]struct{}),
		records: make(map[string]*planRecord), latest: make(map[string]uint64),
	}
	for _, id := range config.AllowedUserIDs {
		a.allowedUsers[id] = struct{}{}
	}
	for _, id := range config.AllowedChatIDs {
		a.allowedChats[id] = struct{}{}
	}
	return a
}

func (a *Approver) SetClock(now func() time.Time) { a.now = now }

func recordKey(id string, version uint64) string { return fmt.Sprintf("%s\x00%d", id, version) }

// Present registers and sends a plan without waiting for its callback.
func (a *Approver) Present(ctx context.Context, plan domain.TradePlan) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	a.mu.Lock()
	key := recordKey(plan.ID(), plan.Version())
	if _, exists := a.records[key]; exists {
		a.mu.Unlock()
		return ErrAlreadyDecided
	}
	a.records[key] = &planRecord{plan: plan, waiter: make(chan decisionResult, 1)}
	a.latest[plan.ID()] = plan.Version()
	a.pendingKey = key
	a.mu.Unlock()
	if a.messenger == nil {
		return nil
	}
	if err := a.messenger.SendPlan(ctx, a.config.SendChatID, RenderPlan(plan)); err != nil {
		a.mu.Lock()
		delete(a.records, key)
		delete(a.latest, plan.ID())
		if a.pendingKey == key {
			a.pendingKey = ""
		}
		a.mu.Unlock()
		return err
	}
	return nil
}

// Decide implements approval.PlanApprover. Telegram update handling should call
// HandleCallback while Decide waits for a decision or context cancellation.
func (a *Approver) Decide(ctx context.Context, plan domain.TradePlan) (domain.Approval, error) {
	if err := a.Present(ctx, plan); err != nil {
		return domain.Approval{}, err
	}
	a.mu.Lock()
	waiter := a.records[recordKey(plan.ID(), plan.Version())].waiter
	a.mu.Unlock()
	select {
	case result := <-waiter:
		return result.approval, result.err
	case <-ctx.Done():
		return domain.Approval{}, ctx.Err()
	}
}

func (a *Approver) HandleCallback(ctx context.Context, callback Callback) (CallbackResult, error) {
	if err := ctx.Err(); err != nil {
		return CallbackResult{}, err
	}
	if !a.authorized(callback.UserID, callback.ChatID) {
		return CallbackResult{}, ErrUnauthorized
	}
	planID, version, action, err := DecodeCallback(callback.Data)
	if err != nil {
		return CallbackResult{}, err
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	latest, exists := a.latest[planID]
	if !exists {
		return CallbackResult{}, ErrUnknownPlan
	}
	if version != latest {
		return CallbackResult{}, ErrStaleVersion
	}
	record, exists := a.records[recordKey(planID, version)]
	if !exists {
		return CallbackResult{}, ErrUnknownPlan
	}
	if !a.now().Before(record.plan.ApproveBy()) {
		return CallbackResult{}, ErrExpiredPlan
	}
	if record.decided {
		return CallbackResult{}, ErrAlreadyDecided
	}

	switch action {
	case ActionApprove, ActionReject:
		decision := domain.ApprovalApproved
		if action == ActionReject {
			decision = domain.ApprovalRejected
		}
		result := domain.Approval{
			PlanID: planID, PlanVersion: version, Decision: decision, DecidedAt: a.now(),
		}
		record.decided = true
		if a.pendingKey == recordKey(planID, version) {
			a.pendingKey = ""
		}
		record.waiter <- decisionResult{approval: result}
		return CallbackResult{Approval: &result}, nil
	case ActionEdit:
		if callback.Edits == nil {
			return CallbackResult{}, ErrEditRequired
		}
		edited, editErr := record.plan.Edit(*callback.Edits)
		if editErr != nil {
			return CallbackResult{}, editErr
		}
		record.decided = true
		record.waiter <- decisionResult{err: ErrPlanEdited}
		a.records[recordKey(edited.ID(), edited.Version())] = &planRecord{
			plan: edited, waiter: make(chan decisionResult, 1),
		}
		a.latest[edited.ID()] = edited.Version()
		a.pendingKey = recordKey(edited.ID(), edited.Version())
		return CallbackResult{EditedPlan: &edited}, nil
	default:
		panic("DecodeCallback accepted unsupported action")
	}
}

func (a *Approver) PendingPlan() (domain.TradePlan, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.pendingKey == "" {
		return domain.TradePlan{}, false
	}
	record, ok := a.records[a.pendingKey]
	if !ok || record.decided {
		return domain.TradePlan{}, false
	}
	return record.plan, true
}

func (a *Approver) authorized(userID, chatID int64) bool {
	_, userOK := a.allowedUsers[userID]
	_, chatOK := a.allowedChats[chatID]
	return userOK && chatOK
}
