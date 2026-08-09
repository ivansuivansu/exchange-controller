# Codex Task 001 — Initial Domain Skeleton

## Goal
Implement the first Go application skeleton according to the project docs.

Do NOT implement Telegram networking or Crypto.com API calls yet.

## Requirements
1. Create packages roughly:
```text
cmd/exchange-controller/
internal/domain/
internal/market/
internal/signal/
internal/idea/
internal/planner/
internal/approval/
internal/execution/
internal/config/
```

2. Implement instrument-agnostic domain types:
- Asset
- Instrument
- Market
- MarketEvent/MarketState
- Signal
- TradeIdea
- TradePlan
- TradePlanStatus
- ApprovalDecision
- EntryExecutionState
- PositionState
- ExecutionState

3. Do not use binary floating point for prices, money, or quantities.

4. Treat TradePlan as immutable approved intent. Execution data must stay separate.

5. ExecutionState must represent:
- requested quantity
- filled quantity
- protected quantity
- average entry price
- entry status
- position status
- unknown/ambiguous state

Add helpers for:
```text
ProtectedQuantity <= FilledQuantity
```
and detection of:
```text
ProtectedQuantity < FilledQuantity
```

6. Define small interfaces:
- MarketDataSource
- SignalDetector
- IdeaBuilder
- TradePlanner
- PlanApprover
- ExecutionEngine

7. Add fake implementations demonstrating:
```text
fake market event
→ signal
→ trade idea
→ trade plan
→ auto approval
→ simulated execution
```

8. Enforce the MVP rule: only one active entry/position lifecycle.

9. Add tests for:
- TradePlan version semantics
- execution quantity invariants
- partial-fill protection risk detection
- single-active-lifecycle rule
- fake pipeline happy path

10. `cmd/exchange-controller/main.go` should run a tiny in-memory demo.

## Non-goals
Do not implement:
- Telegram API
- Crypto.com API
- database
- historical downloader
- real backtest
- complex strategy
- dedicated TradeValidator

## Quality
Run:
```bash
go test ./...
go vet ./...
```

Summarize created files and any architectural deviations.
