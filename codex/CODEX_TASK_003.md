Implement CODEX_TASK_003: public live market data and the first real Signal Engine.

GOAL

The application should observe real BTC_USD public market data from Crypto.com
and run the existing domain pipeline against it.

Execution must remain simulated.
Do NOT implement authenticated Crypto.com trading.

1. Crypto.com public market-data adapter

Add an exchange/cryptocom market-data package.

Implement public BTC_USD market data retrieval using Crypto.com's current
public API.

No API credentials may be required.

Keep Crypto.com-specific DTOs/types inside the adapter.
Convert them into the existing instrument-agnostic domain types.

2. Market metadata

Represent at least:

BTC_USD
Base: BTC
Quote: USD

Do not introduce BTC-specific assumptions into core domain packages.

3. Historical rolling window

Maintain an in-memory rolling price/candle window sufficient for signal
calculation.

Do not add a database yet.

The window size must be configurable.

4. First real Signal Detector

Replace the fake signal logic with an initial configurable
drawdown + recovery detector.

Conceptually:

recent local high
        ↓
price falls by configured drawdown threshold
        ↓
local low is tracked
        ↓
price recovers by configured recovery threshold
        ↓
Signal emitted

Example only:

high = 65,000
low = 64,000

drawdown threshold reached

price recovers from 64,000 to 64,500
→ recovery signal

Do NOT hardcode these example numbers.

5. Detector state

The detector must explicitly model its state.

Suggested conceptual states:

OBSERVING
DRAWDOWN
RECOVERING / SIGNAL_EMITTED

Avoid emitting the same signal repeatedly on every market update.

6. Configuration

Add configuration for at least:

market
lookback/window
drawdown threshold
recovery threshold
signal cooldown / reset behavior
poll/update interval if REST polling is used

Thresholds must use Decimal/fixed-point semantics, not float64.

7. Trade Idea

Convert a real signal into a TradeIdea using the existing IdeaBuilder boundary.

Keep Signal and TradeIdea separate.

8. Planner

Use the existing planner boundary.

It is acceptable for entry/TP/SL formulas to remain simple/configurable
for this iteration.

Do not pretend the strategy is optimized.

9. Telegram integration

When a real Signal → TradeIdea → TradePlan is produced,
present the TradePlan through the Telegram approval layer.

Approval must still route through ExecutionController.

APPROVE
→ SimulationEngine only.

No real order may be submitted.

10. Visible mode safety

Telegram messages and startup logs must make it obvious that execution is:

SIMULATION

not LIVE trading.

11. Network resilience

Handle:
- context cancellation
- HTTP timeout
- non-2xx response
- malformed response
- temporary Crypto.com failure

A temporary market-data failure must not crash the process immediately.
Use bounded retry/backoff behavior.

12. Concurrency

Do not perform slow network calls while holding application/domain mutexes.

Also fix the existing Telegram EDIT path so Messenger.SendPlan is not called
while Approver.mu is held.

Preserve the atomic version transition semantics.

13. Tests

Do not depend on live Crypto.com servers in unit tests.

Use httptest/fakes/fixtures.

Test at least:

- Crypto.com response conversion
- malformed API response
- HTTP error
- rolling-window behavior
- drawdown detection
- recovery detection
- duplicate-signal suppression
- cooldown/reset behavior
- full pipeline:
  market event
  → signal
  → idea
  → plan
  → approval
  → simulated execution

14. Main/application wiring

Add a runnable simulation/live-data mode that uses:

real Crypto.com public market data
+
real SignalDetector
+
Telegram approval
+
SimulationEngine

Do not remove the simple fake/demo path if it remains useful for tests/dev.

15. Non-goals

DO NOT implement:

- Crypto.com authentication
- API keys for trading
- real BUY/SELL
- exchange balances
- subaccount trading
- OCO
- real TP/SL orders
- persistence/database
- sophisticated TradeValidator
- strategy optimization

16. Verification

Run:

go test ./...
go vet ./...

Summarize:
- endpoint/data source chosen
- polling vs websocket choice
- detector state machine
- configuration added
- files created/changed
- any architectural deviations