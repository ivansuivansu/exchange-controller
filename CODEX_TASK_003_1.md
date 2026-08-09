Implement a market-data hardening pass before strategy/planner work.

1. Introduce an instrument-agnostic Candle domain type with:
   Market
   Open
   High
   Low
   Close
   Volume where available
   OpenTime
   CloseTime

2. Extend the Crypto.com public adapter to retrieve BTC_USD candles
   using the current official public candlestick endpoint.

3. The drawdown/recovery strategy must operate on a deterministic candle
   timeframe rather than arbitrary REST ticker sampling.

4. Make candle timeframe configurable.
   Start with 1-minute candles as the MVP default.

5. Do not process the same completed candle twice.

6. Prefer completed candles for strategy decisions.
   Do not let an unfinished candle repeatedly change historical signal state.

7. RollingWindow should store the data actually used by the strategy
   (candles or an appropriate derived market observation).

8. Keep ticker/latest-price support if useful for display and future
   execution, but do not use polling frequency as the strategy timeframe.

9. Preserve the existing detector semantics:
   OBSERVING
   → DRAWDOWN
   → RECOVERING
   → SIGNAL_EMITTED

10. Add tests for:
    - Crypto.com candle conversion
    - candle ordering
    - duplicate candle suppression
    - incomplete/current candle handling
    - rolling candle window
    - drawdown using candle highs
    - local low tracking
    - recovery using subsequent candle prices
    - same historical candle sequence produces the same signal regardless
      of how frequently the API is polled

11. Do not implement authenticated trading, balances, or real execution.

12. Keep current live-data mode clearly marked SIMULATION.

Run:
go test ./...
go vet ./...