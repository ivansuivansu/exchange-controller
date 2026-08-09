package config_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/ivansuivansu/exchange-controller/internal/config"
)

func TestLoadTelegramFromEnv(t *testing.T) {
	t.Setenv("TELEGRAM_BOT_TOKEN", "test-token")
	t.Setenv("TELEGRAM_SEND_CHAT_ID", "-1001")
	t.Setenv("TELEGRAM_ALLOWED_USER_IDS", "10, 20")
	t.Setenv("TELEGRAM_ALLOWED_CHAT_IDS", "-1001,-1002")
	got, err := config.LoadTelegramFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if got.Token != "test-token" || got.SendChatID != -1001 ||
		!reflect.DeepEqual(got.AllowedUserIDs, []int64{10, 20}) ||
		!reflect.DeepEqual(got.AllowedChatIDs, []int64{-1001, -1002}) {
		t.Fatalf("unexpected Telegram config: %+v", got)
	}
}

func TestTelegramValidation(t *testing.T) {
	valid := config.Telegram{
		Token: "token", SendChatID: 10,
		AllowedUserIDs: []int64{20}, AllowedChatIDs: []int64{10},
	}
	tests := []struct {
		name   string
		mutate func(*config.Telegram)
		want   string
	}{
		{"empty token", func(c *config.Telegram) { c.Token = " " }, "BOT_TOKEN"},
		{"missing send chat", func(c *config.Telegram) { c.SendChatID = 0 }, "SEND_CHAT_ID"},
		{"no users", func(c *config.Telegram) { c.AllowedUserIDs = nil }, "authorization"},
		{"only zero user", func(c *config.Telegram) { c.AllowedUserIDs = []int64{0} }, "authorization"},
		{"no chats", func(c *config.Telegram) { c.AllowedChatIDs = nil }, "authorization"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			candidate := valid
			tt.mutate(&candidate)
			err := candidate.Validate()
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Validate() error = %v, want text %q", err, tt.want)
			}
		})
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}
}
