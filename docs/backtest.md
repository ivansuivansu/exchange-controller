# Deterministic Backtesting

Run a cached dataset:

```text
go run ./cmd/exchange-controller backtest --file data/btc_usd_m1.csv
```

Or download an explicit half-open UTC range and cache it:

```text
go run ./cmd/exchange-controller backtest \
  --from 2025-01-01 --to 2025-02-01 \
  --cache data/btc_usd_m1_2025_01.csv \
  --fee-rate 0.001
```

Dates may be `YYYY-MM-DD` or RFC3339. Crypto.com requests are paginated with
`start_ts`, `end_ts`, and `count`. Results are restricted to `[from, to)`,
sorted, deduplicated, validated, and limited to completed candles. The CSV
cache has the fields `timestamp,open,high,low,close,volume`; timestamps are UTC
RFC3339 and decimals are written without floating-point conversion.

Replay time is always the current candle's timestamp; strategy and approval
decisions do not use wall-clock time. By default, completed/expired lifecycle
details are written to `backtest_trades.csv` and monthly results to
`backtest_monthly.csv`. Use `--report path` for a text report or set an export
flag to an empty value to disable that export.

## Placeholder research preset

`PLACEHOLDER_BTC_USD_M1_BASELINE` is a first understandable research baseline,
not an optimized strategy and not a claim of profitability. It uses BTC_USD
M1, a 60-candle lookback, 0.01 (1%) drawdown, 0.005 (0.5%) recovery,
`RECOVERY_CLOSE` entry, `PREVIOUS_HIGH` TP, a stop 0.005 below the local low,
minimum R/R 1.0, 10,000 USD starting capital, and conservative ambiguity.
The full detector, planner, execution, fee, and dataset configuration is
printed before the summary in every report.

## MVP Simulation Rules

- A limit entry submitted after candle N is eligible starting with candle N+1.
- Entry fills when `Low <= planned entry <= High`.
- The current entry type is `LIMIT_BUY`. Its actual fill is exactly the
  approved limit price and can never be worsened above that limit. OHLC data
  is not used to invent price improvement. A future market-entry model may
  apply adverse BUY slippage separately.
- TP/SL evaluation starts on the candle after the entry-fill candle.
- If one later candle touches both TP and SL, `CONSERVATIVE` chooses SL and
  `OPTIMISTIC` chooses TP. The default is `CONSERVATIVE`.
- Adverse configured SELL slippage is subtracted from the selected exit price.
- Entry and exit fees use the configured fractional rate and are rounded away
  from zero to the Decimal scale.
- Entry quantity is the planner-approved quantity and is rounded down by the
  planner before submission.
- An entry is eligible only on candles whose `CloseTime <= EntryExpiresAt`.
  If expiration falls inside a candle, that whole candle is conservatively
  ineligible because OHLC cannot reveal whether the touch happened first.
  A candle closing exactly at expiration remains eligible.

Equity, capital, and maximum drawdown currently use realized closed-trade
capital only. They do not include mark-to-market unrealized position equity.

## Configuration

- `--fee-rate` / `BACKTEST_FEE_RATE` (default `0`)
- `--slippage-rate` / `BACKTEST_SLIPPAGE_RATE` (default `0`)

Fee rate is always printed. It is a simulation input only; no Crypto.com fee
tier is assumed. No parameter search or optimization is performed.
