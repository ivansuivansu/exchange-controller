package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ivansuivansu/exchange-controller/internal/domain"
	"github.com/ivansuivansu/exchange-controller/internal/planner"
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

type Backtest struct {
	CandleCount     int
	FeeRate         domain.Decimal
	SlippageRate    domain.Decimal
	AmbiguityPolicy string
}

func LoadBacktestFromEnv() (Backtest, error) {
	count, err := envInt("BACKTEST_CANDLE_COUNT", 300)
	if err != nil {
		return Backtest{}, err
	}
	fee, err := envDecimal("BACKTEST_FEE_RATE", "0")
	if err != nil {
		return Backtest{}, err
	}
	slippage, err := envDecimal("BACKTEST_SLIPPAGE_RATE", "0")
	if err != nil {
		return Backtest{}, err
	}
	policy := envString("BACKTEST_AMBIGUITY_POLICY", "CONSERVATIVE")
	if count <= 0 || fee.Less(domain.Decimal{}) || slippage.Less(domain.Decimal{}) {
		return Backtest{}, fmt.Errorf("invalid backtest count, fee, or slippage")
	}
	return Backtest{CandleCount: count, FeeRate: fee, SlippageRate: slippage, AmbiguityPolicy: policy}, nil
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
	timeframe := strings.TrimSpace(os.Getenv("MARKET_CANDLE_TIMEFRAME"))
	if timeframe == "" {
		timeframe = "M1"
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
	entryOffset, err := envDecimal("PLANNER_ENTRY_OFFSET", "0")
	if err != nil {
		return LiveDataSimulation{}, err
	}
	retracement, err := envDecimal("PLANNER_RETRACEMENT_TARGET", "0.5")
	if err != nil {
		return LiveDataSimulation{}, err
	}
	slOffset, err := envDecimal("PLANNER_STOP_LOSS_OFFSET", "0.01")
	if err != nil {
		return LiveDataSimulation{}, err
	}
	minimumRR, err := envDecimal("PLANNER_MINIMUM_RISK_REWARD", "1.5")
	if err != nil {
		return LiveDataSimulation{}, err
	}
	reserve, err := envDecimal("TECHNICAL_RESERVE_QUOTE", "0")
	if err != nil {
		return LiveDataSimulation{}, err
	}
	available, err := requiredDecimal("AVAILABLE_QUOTE_CAPITAL")
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
		SignalCooldown: cooldown, PollInterval: poll, CandleTimeframe: timeframe, HTTPTimeout: timeout,
		MaxAttempts: attempts, RetryBackoff: backoff,
		Capital: domain.CapitalSnapshot{QuoteAsset: domain.Asset(quote), AvailableQuote: available, AsOf: time.Now().UTC()},
		Planner: planner.V1Config{
			Name: "planner-v1", EntryMode: planner.EntryMode(envString("PLANNER_ENTRY_MODE", string(planner.EntryRecoveryClose))),
			EntryOffset: entryOffset, TakeProfitMode: planner.TakeProfitMode(envString("PLANNER_TP_MODE", string(planner.TakeProfitPreviousHigh))),
			RetracementTarget: retracement, StopLossOffset: slOffset, MinimumRiskReward: minimumRR,
			ApprovalTTL: approvalTTL, EntryTTL: entryTTL, FixedQuoteReserve: reserve,
		},
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

func envString(name, fallback string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	return value
}

type LiveDataSimulation struct {
	Market            domain.Market
	WindowSize        int
	DrawdownThreshold domain.Decimal
	RecoveryThreshold domain.Decimal
	SignalCooldown    time.Duration
	PollInterval      time.Duration
	CandleTimeframe   string
	HTTPTimeout       time.Duration
	MaxAttempts       int
	RetryBackoff      time.Duration
	Capital           domain.CapitalSnapshot
	Planner           planner.V1Config
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
