package backtest

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/ivansuivansu/exchange-controller/internal/domain"
)

var candleCSVHeader = []string{"timestamp", "open", "high", "low", "close", "volume"}

// NormalizeCandles filters a requested [from, to) range, orders candles, and
// removes duplicate open timestamps. Conflicting duplicates are rejected.
func NormalizeCandles(input []domain.Candle, market domain.Market, timeframe time.Duration, from, to time.Time) ([]domain.Candle, error) {
	if timeframe <= 0 || market.Base == "" || market.Quote == "" || market.Instrument == "" {
		return nil, errors.New("complete dataset market and positive timeframe are required")
	}
	candles := append([]domain.Candle(nil), input...)
	sort.SliceStable(candles, func(i, j int) bool { return candles[i].OpenTime.Before(candles[j].OpenTime) })
	result := make([]domain.Candle, 0, len(candles))
	for _, candle := range candles {
		if !from.IsZero() && candle.OpenTime.Before(from) {
			continue
		}
		if !to.IsZero() && candle.CloseTime.After(to) {
			continue
		}
		if candle.Market != market || candle.OpenTime.IsZero() || candle.CloseTime.Sub(candle.OpenTime) != timeframe || !validOHLC(candle) {
			return nil, errors.New("invalid historical candle")
		}
		if len(result) > 0 && result[len(result)-1].OpenTime.Equal(candle.OpenTime) {
			if result[len(result)-1] != candle {
				return nil, fmt.Errorf("conflicting duplicate candle at %s", candle.OpenTime.Format(time.RFC3339))
			}
			continue
		}
		result = append(result, candle)
	}
	return result, nil
}

func WriteCandleCSV(w io.Writer, candles []domain.Candle) error {
	writer := csv.NewWriter(w)
	if err := writer.Write(candleCSVHeader); err != nil {
		return err
	}
	for _, candle := range candles {
		volume := ""
		if candle.VolumeAvailable {
			volume = candle.Volume.String()
		}
		if err := writer.Write([]string{candle.OpenTime.UTC().Format(time.RFC3339Nano), candle.Open.String(), candle.High.String(), candle.Low.String(), candle.Close.String(), volume}); err != nil {
			return err
		}
	}
	writer.Flush()
	return writer.Error()
}

func SaveCandleCSV(path string, candles []domain.Candle) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := WriteCandleCSV(file, candles); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

func ReadCandleCSV(r io.Reader, market domain.Market, timeframe time.Duration, from, to time.Time) ([]domain.Candle, error) {
	reader := csv.NewReader(r)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(records) == 0 || len(records[0]) != len(candleCSVHeader) {
		return nil, errors.New("invalid candle CSV header")
	}
	for i := range candleCSVHeader {
		if strings.TrimSpace(records[0][i]) != candleCSVHeader[i] {
			return nil, errors.New("invalid candle CSV header")
		}
	}
	candles := make([]domain.Candle, 0, len(records)-1)
	for row, record := range records[1:] {
		if len(record) != len(candleCSVHeader) {
			return nil, fmt.Errorf("candle CSV row %d has %d fields", row+2, len(record))
		}
		at, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(record[0]))
		if err != nil {
			return nil, fmt.Errorf("candle CSV row %d timestamp: %w", row+2, err)
		}
		values := make([]domain.Decimal, 4)
		for i := 0; i < 4; i++ {
			values[i], err = domain.ParseDecimal(strings.TrimSpace(record[i+1]))
			if err != nil {
				return nil, fmt.Errorf("candle CSV row %d price: %w", row+2, err)
			}
		}
		candle := domain.Candle{Market: market, OpenTime: at.UTC(), CloseTime: at.UTC().Add(timeframe), Open: values[0], High: values[1], Low: values[2], Close: values[3]}
		if strings.TrimSpace(record[5]) != "" {
			candle.Volume, err = domain.ParseDecimal(strings.TrimSpace(record[5]))
			if err != nil || candle.Volume.Less(domain.Decimal{}) {
				return nil, fmt.Errorf("candle CSV row %d has invalid volume", row+2)
			}
			candle.VolumeAvailable = true
		}
		candles = append(candles, candle)
	}
	return NormalizeCandles(candles, market, timeframe, from, to)
}

func LoadCandleCSV(path string, market domain.Market, timeframe time.Duration, from, to time.Time) ([]domain.Candle, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return ReadCandleCSV(file, market, timeframe, from, to)
}
