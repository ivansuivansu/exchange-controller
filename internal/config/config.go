package config

import "github.com/ivansuivansu/exchange-controller/internal/domain"

type Mode string

const (
	ModeLive     Mode = "live"
	ModeBacktest Mode = "backtest"
	ModeTrainer  Mode = "trainer"
)

type Config struct {
	Mode   Mode
	Market domain.Market
}
