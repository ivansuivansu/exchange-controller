Perform a backtest execution-semantics hardening pass.

1. Fix LIMIT BUY fill semantics.

A limit BUY must never receive adverse price slippage above its limit.

For the MVP:

if a future candle trades through the limit:
    actual entry price = planned limit price

Fees still apply normally.

Do not model market-order slippage as limit-order slippage.

Structure the code so a future MARKET entry type can have adverse slippage.

2. Review entry expiration at candle boundaries.

OHLC data cannot tell whether an entry was touched before or after an
expiration timestamp that falls inside the candle.

Use a documented conservative rule.

Prefer either:
- require/normalize EntryTTL to candle boundaries, or
- treat the boundary candle conservatively.

Add tests for the chosen behavior.

3. Add tests proving:

- LIMIT BUY actual fill never exceeds its limit
- fees still affect PnL
- an entry cannot fill after expiration
- expiration exactly on a candle boundary behaves deterministically
- expiration inside a candle follows the documented conservative rule

4. Document that current equity/drawdown is based on closed-trade capital,
not mark-to-market unrealized equity.

Do not implement mark-to-market yet.

5. Keep CONSERVATIVE TP/SL ambiguity as the default.

6. Run:

go test ./...
go vet ./...