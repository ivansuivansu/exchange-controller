# Public Market Data Simulation

Run the real-market-data mode with:

```text
APP_MODE=live-data-simulation
```

This mode polls Crypto.com's unauthenticated Exchange REST candlestick endpoint
`/exchange/v1/public/get-candlestick`. Strategy decisions use only completed,
chronologically ordered candles. The ticker endpoint remains available for
future display/execution uses but polling frequency does not define strategy
time. Execution is always `SimulationEngine`; it cannot submit an exchange
order.

## Configuration

- `MARKET_BASE` (default `BTC`)
- `MARKET_QUOTE` (default `USD`)
- `MARKET_INSTRUMENT` (default `BTC_USD`)
- `MARKET_WINDOW_SIZE` (default `60`)
- `MARKET_POLL_INTERVAL` (default `5s`)
- `MARKET_CANDLE_TIMEFRAME` (default `M1`)
- `MARKET_HTTP_TIMEOUT` (default `10s`)
- `MARKET_MAX_ATTEMPTS` (default `3`)
- `MARKET_RETRY_BACKOFF` (default `1s`)
- `SIGNAL_DRAWDOWN_THRESHOLD` (default `0.02`)
- `SIGNAL_RECOVERY_THRESHOLD` (default `0.01`)
- `SIGNAL_COOLDOWN` (default `15m`)
- `AVAILABLE_QUOTE_CAPITAL` (required static simulation snapshot)
- `TECHNICAL_RESERVE_QUOTE` (default `0`)
- `PLANNER_ENTRY_MODE` (`RECOVERY_CLOSE` by default)
- `PLANNER_ENTRY_OFFSET` (default `0`)
- `PLANNER_TP_MODE` (`PREVIOUS_HIGH` by default)
- `PLANNER_RETRACEMENT_TARGET` (default `0.5`)
- `PLANNER_STOP_LOSS_OFFSET` (default `0.01`)
- `PLANNER_MINIMUM_RISK_REWARD` (default `1.5`)
- `PLAN_APPROVAL_TTL` (default `5m`)
- `PLAN_ENTRY_TTL` (default `25m`)

Thresholds, prices, and quantities are parsed as fixed-point `Decimal` values.
The initial detector moves through `observing`, `drawdown`, `recovering`, and
`signal_emitted`; after the configured cooldown it resets to `observing`.

Planner v1 uses a fixed quote-currency reserve. Available capital minus that
reserve is divided by entry price with `Decimal` round-toward-zero semantics.
For positive BUY quantities this is round-down to eight fractional digits, so
the resulting planned cost cannot exceed available quote capital.
