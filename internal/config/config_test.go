package config_test

import (
	"reflect"
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
