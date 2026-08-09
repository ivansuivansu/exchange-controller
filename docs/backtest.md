# Deterministic Backtesting

Run:

```text
go run ./cmd/exchange-controller backtest
```

The command downloads a completed Crypto.com candlestick batch and then
disconnects fetching from replay. Replay time is always the current candle's
timestamp; strategy and approval decisions do not use wall-clock time.

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

- `BACKTEST_CANDLE_COUNT` (default `300`)
- `BACKTEST_FEE_RATE` (default `0`)
- `BACKTEST_SLIPPAGE_RATE` (default `0`)
- `BACKTEST_AMBIGUITY_POLICY` (`CONSERVATIVE` by default)

Starting capital is `AVAILABLE_QUOTE_CAPITAL`. Strategy, planner, market, and
timeframe settings are shared with live-data simulation mode.
