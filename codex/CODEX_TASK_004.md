Implement CODEX_TASK_004: structured recovery signal + parameterized Trade Planner v1.

GOAL

Move from a mostly descriptive signal to a structured trade-planning pipeline.

The system should produce a TradePlan from a real drawdown/recovery signal
without parsing human-readable descriptions.

Execution must remain simulated.
Do NOT implement authenticated Crypto.com trading.

1. Structured signal data

Extend the domain model so a drawdown/recovery Signal carries structured data.

It must expose at least:

- RecentHigh
- LocalLow
- RecoveryPrice
- DrawdownPercent
- RecoveryPercent
- ObservedAt
- Market

Do not encode these values only inside Description strings.

Keep Description only as optional human-readable context.

2. TradeIdea structure

TradeIdea should preserve the structured source context needed by the Planner.

The Planner must not need to look up or parse old Signal descriptions.

A TradeIdea should contain enough information to plan a trade deterministically.

3. Planner v1

Implement a parameterized Planner v1.

The planner must be configuration-driven.

It should calculate:

- Entry price
- Take-profit price
- Stop-loss price
- Planned quote notional
- Planned base quantity
- ApproveBy
- EntryTTL
- Expected gross upside
- Expected downside
- Risk/reward

Do not use float64 for price, money, quantity, or percentages.

4. Entry model

For Planner v1, support a configurable entry mode.

At minimum support:

A. RECOVERY_CLOSE
   Entry = signal RecoveryPrice

B. OFFSET_FROM_RECOVERY
   Entry = RecoveryPrice adjusted by a configurable signed percentage/offset

Do not hardcode one entry rule into domain types.

5. Take-profit model

For Planner v1, support at least:

A. PREVIOUS_HIGH
   TP = signal RecentHigh

B. RETRACEMENT
   TP = configurable fraction/percentage of the move from LocalLow back toward RecentHigh

Example conceptually:

Low = 64,000
High = 65,000

50% retracement target:
64,500

Do not hardcode 50%.

6. Stop-loss model

Support a configurable stop below LocalLow.

Example concept:

SL = LocalLow minus configurable percentage/offset

Do not hardcode the percentage.

7. Risk/reward validation

Planner must reject a plan if:

- Entry <= 0
- TP <= Entry for a long trade
- SL >= Entry
- SL <= 0
- resulting quantity <= 0
- risk/reward is undefined or invalid

Add configurable minimum risk/reward.

If the plan does not satisfy minimum R/R, do not produce an executable plan.

8. Capital model

MVP capital rule remains:

- dedicated subaccount
- up to 100% of available quote balance
- technical reserve/buffer applied

Planner input must receive a CapitalSnapshot or equivalent.

For BTC_USD:
Quote asset = USD

Planner must not assume BTC or USD in core logic.

9. Technical reserve

Add configurable reserve semantics.

Support either:
- reserve as a fixed quote amount
or
- reserve as a percentage of available quote balance

Choose one simple approach for MVP, but keep it explicit in config.

The planner must never create a planned notional above available quote balance.

10. Quantity calculation

Quantity must be derived from:

planned quote notional / entry price

Use decimal-safe arithmetic.

Do not use float64.

Define and document rounding behavior.

For BUY quantity, round down so the planned cost cannot exceed the available quote capital.

11. Decimal arithmetic

If the current Decimal type lacks arithmetic helpers, add only the operations needed for Planner v1.

Requirements:

- overflow-safe behavior
- explicit rounding semantics
- tests for multiplication/division
- no silent precision expansion
- no binary floating point

Avoid overengineering arbitrary-precision math unless necessary.

12. TradePlan fields

TradePlan should expose at least:

- Market
- EntryPrice
- Quantity
- QuoteNotional
- TakeProfit
- StopLoss
- ApproveBy
- EntryTTL
- RiskReward
- GrossUpsidePercent
- DownsidePercent
- Planner name/profile
- Idea ID
- Version

Keep TradePlan immutable.

13. Planner configuration

Add configuration for at least:

- entry mode
- entry offset
- TP mode
- retracement target
- SL offset
- minimum risk/reward
- approval TTL
- entry TTL
- technical capital reserve

All percentage-like values must use Decimal/fixed-point semantics.

14. Telegram rendering

Extend Telegram TradePlan rendering to show:

- Entry
- Planned notional
- Quantity
- TP
- SL
- Gross upside %
- Downside %
- Risk/reward
- ApproveBy
- EntryTTL
- Planner profile/name
- SIMULATION label

Do not add live trading.

15. Planner rejection

If the Planner cannot build a valid plan:

- return a typed/domain error or explicit no-plan result
- do not send an invalid plan to Telegram
- log/report the reason

Examples:

- insufficient capital
- TP <= Entry
- SL >= Entry
- R/R below minimum
- malformed signal context

16. Tests

Add tests for at least:

- structured signal creation
- TradeIdea preserves structured context
- RECOVERY_CLOSE entry
- OFFSET_FROM_RECOVERY entry
- PREVIOUS_HIGH TP
- RETRACEMENT TP
- stop below local low
- invalid TP rejection
- invalid SL rejection
- minimum R/R rejection
- insufficient capital
- technical reserve
- quantity rounds down
- planned cost never exceeds available quote balance
- TradePlan remains immutable
- Telegram rendering includes risk/reward fields
- full pipeline:
  candle
  → structured signal
  → idea
  → planner
  → TradePlan
  → approval
  → simulated execution

17. Non-goals

DO NOT implement:

- authenticated Crypto.com trading
- real balances from exchange
- real subaccount API
- OCO
- live BUY/SELL
- database persistence
- strategy optimization
- automatic parameter search
- sophisticated TradeValidator
- multi-position portfolio allocation

18. Verification

Run:

go test ./...
go vet ./...

Summarize:

- domain changes
- Decimal arithmetic added
- Planner formulas implemented
- rounding policy
- configuration added
- files changed
- any architectural deviations