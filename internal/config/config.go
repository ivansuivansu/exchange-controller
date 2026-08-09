package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/ivansuivansu/exchange-controller/internal/domain"
)

type Mode string

const (
	ModeLive     Mode = "live"
	ModeBacktest Mode = "backtest"
	ModeTrainer  Mode = "trainer"
)

type Config struct {
	Mode     Mode
	Market   domain.Market
	Telegram Telegram
}

type Telegram struct {
	Token          string
	SendChatID     int64
	AllowedUserIDs []int64
	AllowedChatIDs []int64
}

func LoadTelegramFromEnv() (Telegram, error) {
	users, err := parseIDs(os.Getenv("TELEGRAM_ALLOWED_USER_IDS"))
	if err != nil {
		return Telegram{}, fmt.Errorf("TELEGRAM_ALLOWED_USER_IDS: %w", err)
	}
	chats, err := parseIDs(os.Getenv("TELEGRAM_ALLOWED_CHAT_IDS"))
	if err != nil {
		return Telegram{}, fmt.Errorf("TELEGRAM_ALLOWED_CHAT_IDS: %w", err)
	}
	sendChatID, err := strconv.ParseInt(strings.TrimSpace(os.Getenv("TELEGRAM_SEND_CHAT_ID")), 10, 64)
	if err != nil {
		return Telegram{}, fmt.Errorf("TELEGRAM_SEND_CHAT_ID: %w", err)
	}
	return Telegram{
		Token: os.Getenv("TELEGRAM_BOT_TOKEN"), SendChatID: sendChatID,
		AllowedUserIDs: users, AllowedChatIDs: chats,
	}, nil
}

func parseIDs(value string) ([]int64, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	parts := strings.Split(value, ",")
	ids := make([]int64, 0, len(parts))
	for _, part := range parts {
		id, err := strconv.ParseInt(strings.TrimSpace(part), 10, 64)
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}
