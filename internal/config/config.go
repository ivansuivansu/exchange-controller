package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

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

func LoadLiveDataSimulationFromEnv() (LiveDataSimulation, error) {
	base := strings.TrimSpace(os.Getenv("MARKET_BASE"))
	if base == "" {
		base = "BTC"
	}
	quote := strings.TrimSpace(os.Getenv("MARKET_QUOTE"))
	if quote == "" {
		quote = "USD"
	}
	instrument := strings.TrimSpace(os.Getenv("MARKET_INSTRUMENT"))
	if instrument == "" {
		instrument = "BTC_USD"
	}
	window, err := envInt("MARKET_WINDOW_SIZE", 60)
	if err != nil {
		return LiveDataSimulation{}, err
	}
	attempts, err := envInt("MARKET_MAX_ATTEMPTS", 3)
	if err != nil {
		return LiveDataSimulation{}, err
	}
	drawdown, err := envDecimal("SIGNAL_DRAWDOWN_THRESHOLD", "0.02")
	if err != nil {
		return LiveDataSimulation{}, err
	}
	recovery, err := envDecimal("SIGNAL_RECOVERY_THRESHOLD", "0.01")
	if err != nil {
		return LiveDataSimulation{}, err
	}
	quantity, err := envDecimal("PLANNER_QUANTITY", "0.001")
	if err != nil {
		return LiveDataSimulation{}, err
	}
	tp, err := requiredDecimal("PLANNER_TAKE_PROFIT")
	if err != nil {
		return LiveDataSimulation{}, err
	}
	sl, err := requiredDecimal("PLANNER_STOP_LOSS")
	if err != nil {
		return LiveDataSimulation{}, err
	}
	poll, err := envDuration("MARKET_POLL_INTERVAL", 5*time.Second)
	if err != nil {
		return LiveDataSimulation{}, err
	}
	timeout, err := envDuration("MARKET_HTTP_TIMEOUT", 10*time.Second)
	if err != nil {
		return LiveDataSimulation{}, err
	}
	backoff, err := envDuration("MARKET_RETRY_BACKOFF", time.Second)
	if err != nil {
		return LiveDataSimulation{}, err
	}
	cooldown, err := envDuration("SIGNAL_COOLDOWN", 15*time.Minute)
	if err != nil {
		return LiveDataSimulation{}, err
	}
	approvalTTL, err := envDuration("PLAN_APPROVAL_TTL", 5*time.Minute)
	if err != nil {
		return LiveDataSimulation{}, err
	}
	entryTTL, err := envDuration("PLAN_ENTRY_TTL", 25*time.Minute)
	if err != nil {
		return LiveDataSimulation{}, err
	}
	if window <= 1 || attempts <= 0 {
		return LiveDataSimulation{}, fmt.Errorf("window must exceed one and attempts must be positive")
	}
	return LiveDataSimulation{
		Market: domain.Market{
			Base: domain.Asset(base), Quote: domain.Asset(quote), Instrument: domain.Instrument(instrument),
		},
		WindowSize: window, DrawdownThreshold: drawdown, RecoveryThreshold: recovery,
		SignalCooldown: cooldown, PollInterval: poll, HTTPTimeout: timeout,
		MaxAttempts: attempts, RetryBackoff: backoff, Quantity: quantity,
		TakeProfit: tp, StopLoss: sl, ApprovalTTL: approvalTTL, EntryTTL: entryTTL,
	}, nil
}

func envInt(name string, fallback int) (int, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", name, err)
	}
	return parsed, nil
}

func envDuration(name string, fallback time.Duration) (time.Duration, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback, nil
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", name, err)
	}
	return parsed, nil
}

func envDecimal(name, fallback string) (domain.Decimal, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		value = fallback
	}
	parsed, err := domain.ParseDecimal(value)
	if err != nil {
		return domain.Decimal{}, fmt.Errorf("%s: %w", name, err)
	}
	return parsed, nil
}

func requiredDecimal(name string) (domain.Decimal, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return domain.Decimal{}, fmt.Errorf("%s must be configured", name)
	}
	return envDecimal(name, "")
}

type LiveDataSimulation struct {
	Market            domain.Market
	WindowSize        int
	DrawdownThreshold domain.Decimal
	RecoveryThreshold domain.Decimal
	SignalCooldown    time.Duration
	PollInterval      time.Duration
	HTTPTimeout       time.Duration
	MaxAttempts       int
	RetryBackoff      time.Duration
	Quantity          domain.Decimal
	TakeProfit        domain.Decimal
	StopLoss          domain.Decimal
	ApprovalTTL       time.Duration
	EntryTTL          time.Duration
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
	config := Telegram{
		Token: os.Getenv("TELEGRAM_BOT_TOKEN"), SendChatID: sendChatID,
		AllowedUserIDs: users, AllowedChatIDs: chats,
	}
	if err := config.Validate(); err != nil {
		return Telegram{}, err
	}
	return config, nil
}

// Validate checks configuration required when Telegram operation is enabled.
func (c Telegram) Validate() error {
	if strings.TrimSpace(c.Token) == "" {
		return fmt.Errorf("TELEGRAM_BOT_TOKEN must not be empty")
	}
	if c.SendChatID == 0 {
		return fmt.Errorf("TELEGRAM_SEND_CHAT_ID must be configured")
	}
	if !hasNonZeroID(c.AllowedUserIDs) || !hasNonZeroID(c.AllowedChatIDs) {
		return fmt.Errorf("Telegram authorization requires at least one non-zero user ID and chat ID")
	}
	return nil
}

func hasNonZeroID(ids []int64) bool {
	for _, id := range ids {
		if id != 0 {
			return true
		}
	}
	return false
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
