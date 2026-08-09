package telegram

import (
	"context"

	"github.com/ivansuivansu/exchange-controller/internal/domain"
)

type Action string

const (
	ActionApprove Action = "approve"
	ActionEdit    Action = "edit"
	ActionReject  Action = "reject"
)

type InlineButton struct {
	Text         string
	CallbackData string
}

type PlanMessage struct {
	Text    string
	Buttons []InlineButton
}

// Messenger is the Telegram transport boundary. A future network adapter can
// implement it without leaking Telegram types into the domain package.
type Messenger interface {
	SendPlan(context.Context, int64, PlanMessage) error
	SendText(context.Context, int64, string) error
}

type Callback struct {
	UserID int64
	ChatID int64
	Data   string
	Edits  *domain.TradePlanEdits
}

type CallbackResult struct {
	Approval   *domain.Approval
	EditedPlan *domain.TradePlan
}

type StatusSource interface {
	Current() (domain.ExecutionState, bool)
}

type Config struct {
	SendChatID     int64
	AllowedUserIDs []int64
	AllowedChatIDs []int64
	Mode           string
}
