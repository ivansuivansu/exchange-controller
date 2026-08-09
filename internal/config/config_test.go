package config_test

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/ivansuivansu/exchange-controller/internal/config"
	"github.com/ivansuivansu/exchange-controller/internal/planner"
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

func TestLoadLiveDataSimulationFromEnv(t *testing.T) {
	t.Setenv("AVAILABLE_QUOTE_CAPITAL", "1000")
	t.Setenv("MARKET_WINDOW_SIZE", "42")
	t.Setenv("MARKET_BASE", "ETH")
	t.Setenv("MARKET_QUOTE", "USD")
	t.Setenv("MARKET_INSTRUMENT", "ETH_USD")
	t.Setenv("SIGNAL_DRAWDOWN_THRESHOLD", "0.03")
	t.Setenv("SIGNAL_RECOVERY_THRESHOLD", "0.015")
	t.Setenv("MARKET_POLL_INTERVAL", "2s")
	got, err := config.LoadLiveDataSimulationFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if got.Market.Base != "ETH" || got.Market.Quote != "USD" || got.Market.Instrument != "ETH_USD" || got.WindowSize != 42 ||
		got.DrawdownThreshold.String() != "0.03" || got.RecoveryThreshold.String() != "0.015" ||
		got.PollInterval != 2*time.Second || got.CandleTimeframe != "M1" {
		t.Fatalf("unexpected live-data configuration: %+v", got)
	}
	if got.Capital.QuoteAsset != "USD" || got.Capital.AvailableQuote.String() != "1000" ||
		got.Planner.EntryMode != planner.EntryRecoveryClose || got.Planner.TakeProfitMode != planner.TakeProfitPreviousHigh {
		t.Fatalf("unexpected planner/capital configuration: %+v %+v", got.Planner, got.Capital)
	}
}

func TestLoadBacktestFromEnv(t *testing.T) {
	t.Setenv("BACKTEST_CANDLE_COUNT", "50")
	t.Setenv("BACKTEST_FEE_RATE", "0.001")
	t.Setenv("BACKTEST_SLIPPAGE_RATE", "0.002")
	t.Setenv("BACKTEST_AMBIGUITY_POLICY", "OPTIMISTIC")
	got, err := config.LoadBacktestFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if got.CandleCount != 50 || got.FeeRate.String() != "0.001" || got.SlippageRate.String() != "0.002" || got.AmbiguityPolicy != "OPTIMISTIC" {
		t.Fatalf("unexpected backtest config: %+v", got)
	}
}
