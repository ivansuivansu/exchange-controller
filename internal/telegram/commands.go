package telegram

import (
	"context"
	"fmt"
)

func (a *Approver) HandleCommand(ctx context.Context, userID, chatID int64, command string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if !a.authorized(userID, chatID) {
		return "", ErrUnauthorized
	}
	var response string
	switch command {
	case "/start":
		response = "Exchange Controller is ready. Use /status to inspect current state."
	case "/status":
		activeText := "none"
		if a.status != nil {
			if state, ok := a.status.Current(); ok && state.LifecycleActive() {
				activeText = fmt.Sprintf("%s v%d (%s, %s)", state.PlanID, state.PlanVersion, state.EntryStatus, state.PositionStatus)
			}
		}
		pendingText := "none"
		if plan, ok := a.PendingPlan(); ok {
			pendingText = fmt.Sprintf("%s v%d", plan.ID(), plan.Version())
		}
		response = fmt.Sprintf("Mode: %s\nActive lifecycle: %s\nPending plan: %s", a.config.Mode, activeText, pendingText)
	default:
		return "", ErrUnsupportedCommand
	}
	if a.messenger != nil {
		if err := a.messenger.SendText(ctx, chatID, response); err != nil {
			return "", err
		}
	}
	return response, nil
}
