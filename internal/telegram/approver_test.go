package telegram_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ivansuivansu/exchange-controller/internal/domain"
	"github.com/ivansuivansu/exchange-controller/internal/telegram"
)

const (
	allowedUser int64 = 101
	allowedChat int64 = 202
)

type fakeMessenger struct {
	mu    sync.Mutex
	plans chan telegram.PlanMessage
	texts []string
}

type blockingEditMessenger struct {
	calls   int
	started chan struct{}
	release chan struct{}
}

func (m *blockingEditMessenger) SendPlan(_ context.Context, _ int64, _ telegram.PlanMessage) error {
	m.calls++
	if m.calls == 2 {
		close(m.started)
		<-m.release
	}
	return nil
}

func (*blockingEditMessenger) SendText(context.Context, int64, string) error { return nil }

func newFakeMessenger() *fakeMessenger {
	return &fakeMessenger{plans: make(chan telegram.PlanMessage, 10)}
}

func (m *fakeMessenger) SendPlan(_ context.Context, _ int64, message telegram.PlanMessage) error {
	m.plans <- message
	return nil
}

func (m *fakeMessenger) SendText(_ context.Context, _ int64, text string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.texts = append(m.texts, text)
	return nil
}

func testPlan(t *testing.T, now time.Time) domain.TradePlan {
	t.Helper()
	plan, err := domain.NewTradePlan(domain.TradePlanParams{
		ID: "plan:with/safe ID", Version: 1, IdeaID: "idea-1",
		Market:     domain.Market{Base: "BTC", Quote: "USD", Instrument: "BTC_USD"},
		EntryPrice: domain.MustDecimal("64100"), Quantity: domain.MustDecimal("0.01"),
		TakeProfit: domain.MustDecimal("65800"), StopLoss: domain.MustDecimal("63500"),
		ApproveBy: now.Add(time.Minute), EntryTTL: 25 * time.Minute,
		QuoteNotional: domain.MustDecimal("641"), RiskReward: domain.MustDecimal("2"),
		GrossUpsidePercent: domain.MustDecimal("0.02"), DownsidePercent: domain.MustDecimal("0.01"), PlannerName: "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	return plan
}

func newApprover(now time.Time, messenger telegram.Messenger) *telegram.Approver {
	a := telegram.NewApprover(telegram.Config{
		SendChatID: allowedChat, AllowedUserIDs: []int64{allowedUser},
		AllowedChatIDs: []int64{allowedChat}, Mode: "trainer",
	}, messenger, nil)
	a.SetClock(func() time.Time { return now })
	return a
}

func callback(plan domain.TradePlan, action telegram.Action) telegram.Callback {
	return telegram.Callback{
		UserID: allowedUser, ChatID: allowedChat,
		Data: telegram.EncodeCallback(plan.ID(), plan.Version(), action),
	}
}

func TestAuthorizedApproval(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	messenger := newFakeMessenger()
	approver := newApprover(now, messenger)
	plan := testPlan(t, now)
	type outcome struct {
		approval domain.Approval
		err      error
	}
	done := make(chan outcome, 1)
	go func() {
		decision, err := approver.Decide(context.Background(), plan)
		done <- outcome{decision, err}
	}()
	message := <-messenger.plans
	if len(message.Buttons) != 3 {
		t.Fatalf("button count = %d", len(message.Buttons))
	}
	result, err := approver.HandleCallback(context.Background(), callback(plan, telegram.ActionApprove))
	if err != nil {
		t.Fatal(err)
	}
	got := <-done
	if got.err != nil {
		t.Fatal(got.err)
	}
	if got.approval.Decision != domain.ApprovalApproved || result.Approval == nil || !got.approval.AppliesTo(plan) {
		t.Fatalf("unexpected approval: %+v", got.approval)
	}
}

func TestUnauthorizedApproval(t *testing.T) {
	now := time.Now()
	a := newApprover(now, nil)
	plan := testPlan(t, now)
	if err := a.Present(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
	request := callback(plan, telegram.ActionApprove)
	request.UserID++
	if _, err := a.HandleCallback(context.Background(), request); !errors.Is(err, telegram.ErrUnauthorized) {
		t.Fatalf("error = %v, want ErrUnauthorized", err)
	}
}

func TestStaleVersionCallback(t *testing.T) {
	now := time.Now()
	a := newApprover(now, nil)
	plan := testPlan(t, now)
	if err := a.Present(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
	request := callback(plan, telegram.ActionApprove)
	request.Data = telegram.EncodeCallback(plan.ID(), plan.Version()+1, telegram.ActionApprove)
	if _, err := a.HandleCallback(context.Background(), request); !errors.Is(err, telegram.ErrStaleVersion) {
		t.Fatalf("error = %v, want ErrStaleVersion", err)
	}
}

func TestUnknownPlanCallback(t *testing.T) {
	now := time.Now()
	a := newApprover(now, nil)
	request := telegram.Callback{
		UserID: allowedUser, ChatID: allowedChat,
		Data: telegram.EncodeCallback("unknown", 1, telegram.ActionApprove),
	}
	if _, err := a.HandleCallback(context.Background(), request); !errors.Is(err, telegram.ErrUnknownPlan) {
		t.Fatalf("error = %v, want ErrUnknownPlan", err)
	}
}

func TestExpiredPlan(t *testing.T) {
	now := time.Now()
	a := newApprover(now.Add(time.Minute), nil)
	plan := testPlan(t, now)
	if err := a.Present(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
	if _, err := a.HandleCallback(context.Background(), callback(plan, telegram.ActionApprove)); !errors.Is(err, telegram.ErrExpiredPlan) {
		t.Fatalf("error = %v, want ErrExpiredPlan", err)
	}
}

func TestDuplicateApproval(t *testing.T) {
	now := time.Now()
	a := newApprover(now, nil)
	plan := testPlan(t, now)
	if err := a.Present(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
	request := callback(plan, telegram.ActionApprove)
	if _, err := a.HandleCallback(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if _, err := a.HandleCallback(context.Background(), request); !errors.Is(err, telegram.ErrAlreadyDecided) {
		t.Fatalf("error = %v, want ErrAlreadyDecided", err)
	}
}

func TestRejection(t *testing.T) {
	now := time.Now()
	a := newApprover(now, nil)
	plan := testPlan(t, now)
	if err := a.Present(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
	result, err := a.HandleCallback(context.Background(), callback(plan, telegram.ActionReject))
	if err != nil {
		t.Fatal(err)
	}
	if result.Approval == nil || result.Approval.Decision != domain.ApprovalRejected {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestApprovalDoesNotMutateTradePlan(t *testing.T) {
	now := time.Now()
	a := newApprover(now, nil)
	plan := testPlan(t, now)
	originalEntry := plan.EntryPrice()
	originalVersion := plan.Version()
	if err := a.Present(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
	if _, err := a.HandleCallback(context.Background(), callback(plan, telegram.ActionApprove)); err != nil {
		t.Fatal(err)
	}
	if plan.Version() != originalVersion || !plan.EntryPrice().Equal(originalEntry) {
		t.Fatal("approval mutated TradePlan intent")
	}
}

func TestEditCreatesNewVersionWithoutMutatingOriginal(t *testing.T) {
	now := time.Now()
	messenger := newFakeMessenger()
	a := newApprover(now, messenger)
	plan := testPlan(t, now)
	if err := a.Present(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
	<-messenger.plans
	entry := domain.MustDecimal("64000")
	request := callback(plan, telegram.ActionEdit)
	request.Edits = &domain.TradePlanEdits{EntryPrice: &entry}
	result, err := a.HandleCallback(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if result.EditedPlan == nil || result.EditedPlan.Version() != plan.Version()+1 {
		t.Fatalf("unexpected edited plan: %+v", result.EditedPlan)
	}
	editedMessage := <-messenger.plans
	if !strings.Contains(editedMessage.Text, "v2") || len(editedMessage.Buttons) != 3 {
		t.Fatalf("edited plan was not presented for fresh approval: %+v", editedMessage)
	}
	if !result.EditedPlan.EntryPrice().Equal(entry) {
		t.Fatal("entry edit was not applied")
	}
	if !plan.EntryPrice().Equal(domain.MustDecimal("64100")) || plan.Version() != 1 {
		t.Fatal("approval workflow mutated original TradePlan")
	}
	if pending, ok := a.PendingPlan(); !ok || pending.Version() != 2 {
		t.Fatal("edited version is not the pending plan")
	}
	if _, err := a.HandleCallback(context.Background(), callback(plan, telegram.ActionApprove)); !errors.Is(err, telegram.ErrStaleVersion) {
		t.Fatalf("old callback error = %v, want ErrStaleVersion", err)
	}
}

func TestOnlyOnePendingPlanAtATime(t *testing.T) {
	now := time.Now()
	a := newApprover(now, nil)
	first := testPlan(t, now)
	if err := a.Present(context.Background(), first); err != nil {
		t.Fatal(err)
	}
	entry := domain.MustDecimal("64000")
	second, err := first.Edit(domain.TradePlanEdits{EntryPrice: &entry})
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Present(context.Background(), second); !errors.Is(err, telegram.ErrPendingPlan) {
		t.Fatalf("second pending plan error = %v, want ErrPendingPlan", err)
	}
	if pending, ok := a.PendingPlan(); !ok || !pending.IsVersion(first.ID(), first.Version()) {
		t.Fatal("first plan was not preserved as the sole pending plan")
	}
}

func TestEditDoesNotHoldApproverMutexDuringSend(t *testing.T) {
	now := time.Now()
	messenger := &blockingEditMessenger{started: make(chan struct{}), release: make(chan struct{})}
	a := newApprover(now, messenger)
	plan := testPlan(t, now)
	if err := a.Present(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
	entry := domain.MustDecimal("64000")
	request := callback(plan, telegram.ActionEdit)
	request.Edits = &domain.TradePlanEdits{EntryPrice: &entry}
	done := make(chan error, 1)
	go func() {
		_, err := a.HandleCallback(context.Background(), request)
		done <- err
	}()
	<-messenger.started
	pendingRead := make(chan domain.TradePlan, 1)
	go func() {
		pending, _ := a.PendingPlan()
		pendingRead <- pending
	}()
	select {
	case pending := <-pendingRead:
		if !pending.IsVersion(plan.ID(), plan.Version()) {
			t.Fatal("old plan not visible during edit transition")
		}
	case <-time.After(time.Second):
		t.Fatal("PendingPlan blocked on slow Messenger.SendPlan")
	}
	close(messenger.release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if pending, ok := a.PendingPlan(); !ok || pending.Version() != plan.Version()+1 {
		t.Fatal("edited plan not committed atomically")
	}
}

func TestRenderPlanAndCallbackRoundTrip(t *testing.T) {
	now := time.Now()
	plan := testPlan(t, now)
	message := telegram.RenderPlan(plan)
	for _, value := range []string{
		"SIMULATION", "BTC_USD", "Planner: test", "64100", "Planned notional: 641 USD",
		"0.01 BTC", "65800", "63500", "Gross upside: 2%", "Downside: 1%",
		"Risk/reward: 2", plan.ID(), "v1", "25m0s",
	} {
		if !strings.Contains(message.Text, value) {
			t.Errorf("rendered plan lacks %q", value)
		}
	}
	for _, button := range message.Buttons {
		id, version, _, err := telegram.DecodeCallback(button.CallbackData)
		if err != nil || id != plan.ID() || version != plan.Version() {
			t.Fatalf("invalid callback round trip: %q, %v", button.CallbackData, err)
		}
	}
}

func TestCommands(t *testing.T) {
	now := time.Now()
	messenger := newFakeMessenger()
	a := newApprover(now, messenger)
	plan := testPlan(t, now)
	if err := a.Present(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
	<-messenger.plans
	status, err := a.HandleCommand(context.Background(), allowedUser, allowedChat, "/status")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(status, "Mode: trainer") || !strings.Contains(status, plan.ID()+" v1") || !strings.Contains(status, "Active lifecycle: none") {
		t.Fatalf("unexpected status: %s", status)
	}
	if _, err := a.HandleCommand(context.Background(), allowedUser, allowedChat, "/start"); err != nil {
		t.Fatal(err)
	}
}
