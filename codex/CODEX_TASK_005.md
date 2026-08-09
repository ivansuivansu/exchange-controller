Implement CODEX_TASK_005: deterministic historical backtesting engine.

GOAL

Run the existing candle → signal → idea → Planner v1 pipeline against
historical BTC_USD candles and simulate complete trade lifecycles.

The backtester must reuse production strategy/planner code.
Do not create separate "backtest strategy" logic.

No authenticated Crypto.com trading.

1. Historical candle source

Create a HistoricalCandleSource abstraction.

It must provide domain.Candle values in chronological order.

Support an in-memory implementation for tests.

Add a Crypto.com historical candle loader using the existing public
candlestick API where practical.

Keep download/fetch concerns separate from replay.

2. Deterministic replay

Backtest results must depend only on:

- historical candles
- strategy configuration
- planner configuration
- simulation configuration

Do not use time.Now() inside backtest decision logic.

Market/candle timestamps are the simulation clock.

3. Backtest pipeline

Reuse:

Historical candles
→ existing SignalDetector
→ existing IdeaBuilder
→ existing PlannerV1
→ automatic approval
→ BacktestExecutionEngine

Do not duplicate SignalDetector or Planner logic.

4. Automatic approval

Backtest mode automatically approves every valid TradePlan.

Approval timestamp should use simulated market time.

5. Capital

Add a configurable starting quote balance.

Example:

starting capital = 10,000 USD

MVP still uses the existing capital rule:
up to 100% available quote balance minus configured technical reserve.

After every closed trade, update simulated quote capital based on
actual simulated proceeds and fees.

This naturally produces compounding.

6. Entry simulation

For a LIMIT BUY plan:

After the plan is created/approved, inspect only FUTURE candles.

Never use the signal candle to fill an order created after that candle.

The entry fills when a future candle trades through the limit price.

For a candle with:

Low <= EntryPrice <= High

the entry may be considered filled according to an explicitly documented
MVP fill rule.

Do not assume fills using future information before the candle occurs.

7. Entry expiration

Use TradePlan.EntryTTL.

If the entry is not filled before expiration:

- mark entry expired
- release capital
- record the plan as unfilled
- wait for the next eligible signal

8. Position simulation

After entry fill, simulate TP and SL using FUTURE candles.

For a long position:

TP hit when candle High >= TakeProfit
SL hit when candle Low <= StopLoss

9. Ambiguous candle handling

Critical:

A single OHLC candle may touch BOTH TP and SL.

OHLC data does not reveal which happened first.

Implement a configurable ambiguity policy.

Support at least:

CONSERVATIVE:
assume SL happened first

OPTIMISTIC:
assume TP happened first

For reported primary results, default to CONSERVATIVE.

Track the number of ambiguous candles/trades separately.

Do not silently choose TP.

10. No same-candle lookahead

Be explicit about order timing.

If an entry fills inside a candle, do not use impossible knowledge about
earlier parts of that same candle.

For MVP, choose and document a conservative rule.

Preferred simple rule:

- entry may fill on candle N
- TP/SL evaluation begins on candle N+1

This avoids inventing intrabar ordering.

11. Fees

Add configurable trading fee rate.

Apply fees to:

- entry
- exit

Fee configuration uses Decimal ratio semantics:

0.001 = 0.1%

Do not hardcode Crypto.com fee assumptions as strategy truth.

12. Slippage

Add configurable slippage model.

For MVP a fixed percentage is sufficient.

Allow zero slippage.

Apply adverse slippage:

BUY:
effective fill price >= planned entry

SELL:
effective exit price <= planned exit

Keep limit-order semantics conservative and documented.

13. Trade result

Create a BacktestTradeResult containing at least:

- PlanID
- IdeaID
- Instrument
- SignalTime
- EntrySubmittedAt
- EntryFilledAt
- EntryPricePlanned
- EntryPriceActual
- ExitAt
- ExitReason (TP / SL / expiration / other)
- ExitPrice
- Quantity
- EntryFee
- ExitFee
- GrossPnL
- NetPnL
- ReturnPercent
- CapitalBefore
- CapitalAfter
- WasAmbiguous

14. Backtest report

Produce aggregate metrics:

- starting capital
- ending capital
- net profit/loss
- total return %
- total signals
- plans produced
- planner rejections
- entries filled
- entries expired/unfilled
- winning trades
- losing trades
- win rate
- average win
- average loss
- average trade return
- largest win
- largest loss
- profit factor
- maximum drawdown
- ambiguous trade count
- total fees paid

15. Equity curve

Track account equity/capital after every closed trade.

Provide data suitable for later charting.

No UI/chart library is required yet.

16. One-active-lifecycle rule

Backtest must respect the same MVP rule as live mode:

while an entry is waiting or a position is open,
do not execute another TradePlan.

Signals may optionally be counted for analytics, but must not create
simultaneous positions.

17. Planner rejection analytics

Do not treat Planner rejection as a system error.

Count rejection reasons such as:

- minimum risk/reward
- insufficient capital
- invalid TP
- invalid SL

Include counts in the report.

18. Historical data safety

Ensure candles are:

- sorted chronologically
- deduplicated
- valid OHLC
- for the expected market/timeframe

Reject malformed historical sequences explicitly.

19. Tests

Add deterministic tests for at least:

- entry fill
- entry expiration
- TP win
- SL loss
- both TP and SL touched in same candle
- conservative ambiguity behavior
- optimistic ambiguity behavior
- no same-candle TP/SL after entry
- entry and exit fees
- slippage
- capital compounding
- one-active-position rule
- planner rejection counting
- maximum drawdown
- profit factor
- deterministic replay produces identical result twice
- no future candle is observed before its simulated time

20. CLI

Add a backtest command/mode.

Example conceptually:

go run ./cmd/exchange-controller backtest ...

It should print a concise report.

Do not overbuild CLI UX.

21. Percentage convention

Document globally:

All ratio/percentage Decimal values use fractional representation:

1     = 100%
0.1   = 10%
0.01  = 1%
0.001 = 0.1%

Telegram/report rendering may multiply by 100 for human display.

22. Non-goals

DO NOT implement:

- parameter optimization
- grid search
- machine learning
- authenticated Crypto.com trading
- real balances
- real orders
- database
- multi-position portfolio logic
- short selling
- leverage

23. Verification

Run:

go test ./...
go vet ./...

Summarize:

- historical source design
- simulation clock design
- entry fill rule
- same-candle rule
- ambiguity policy
- fee/slippage model
- report metrics
- files created/changed
- architectural deviations