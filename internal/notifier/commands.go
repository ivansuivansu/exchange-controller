package notifier

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var (
	ErrUnauthorized       = errors.New("unauthorized Telegram actor")
	ErrUnsupportedCommand = errors.New("unsupported command")
)

type CommandHandler struct {
	monitor      *Monitor
	sender       TextSender
	allowedUsers map[int64]struct{}
	allowedChats map[int64]struct{}
}

func NewCommandHandler(monitor *Monitor, sender TextSender, users, chats []int64) *CommandHandler {
	h := &CommandHandler{monitor: monitor, sender: sender, allowedUsers: make(map[int64]struct{}), allowedChats: make(map[int64]struct{})}
	for _, id := range users {
		h.allowedUsers[id] = struct{}{}
	}
	for _, id := range chats {
		h.allowedChats[id] = struct{}{}
	}
	return h
}

func (h *CommandHandler) HandleCommand(ctx context.Context, userID, chatID int64, command string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if _, ok := h.allowedUsers[userID]; !ok {
		return "", ErrUnauthorized
	}
	if _, ok := h.allowedChats[chatID]; !ok {
		return "", ErrUnauthorized
	}
	var response string
	switch command {
	case "/start":
		response = "BTC decline notifier is ready. Use /status to inspect current state."
	case "/status":
		response = RenderStatus(h.monitor.Snapshot())
	default:
		return "", ErrUnsupportedCommand
	}
	if h.sender != nil {
		if err := h.sender.SendText(ctx, chatID, response); err != nil {
			return "", err
		}
	}
	return response, nil
}

func RenderStatus(s Snapshot) string {
	value := func(ready bool, decimal fmt.Stringer) string {
		if !ready {
			return "waiting for samples"
		}
		return decimal.String()
	}
	lastAlert := "never"
	if !s.LastAlert.IsZero() {
		lastAlert = s.LastAlert.UTC().Format(time.RFC3339)
	}
	return fmt.Sprintf("Notifier: ON\nInstrument: %s\nPoll interval: %s\nAverage window: %d samples (~%s)\nComparison period: ~%s\nDrop threshold: %s\nCooldown: %s\nCurrent price: %s\nPrevious average: %s\nCurrent average: %s\nCurrent change: %s\nLast alert: %s",
		s.Config.Market.Instrument, s.Config.PollInterval, s.Config.WindowSize, approximate(time.Duration(s.Config.WindowSize)*s.Config.PollInterval),
		approximate(time.Duration(2*s.Config.WindowSize)*s.Config.PollInterval), Percent(s.Config.DropThreshold), s.Config.Cooldown,
		value(!s.CurrentPrice.IsZero(), s.CurrentPrice), value(s.Ready, s.PreviousAverage), value(s.Ready, s.CurrentAverage),
		func() string {
			if !s.Ready {
				return "waiting for samples"
			}
			return Percent(s.CurrentChange)
		}(), lastAlert)
}
