# Architecture

```text
MarketDataSource
→ SignalDetector
→ Signal
→ IdeaBuilder
→ TradeIdea
→ TradePlanner
→ TradePlan(s)
→ PlanApprover
→ ExecutionController
→ ExecutionEngine
```

## Mode Composition
Backtest = HistoricalFeed + AutoApprover + SimulationExecution  
Trainer = HistoricalFeed + TelegramApprover + SimulationExecution  
Live = CryptoComLiveFeed + TelegramApprover + CryptoComExecution

## Instrument-Agnostic Core
Avoid BTC-specific domain types.

Conceptually:
```go
type Asset string
type Instrument string

type Market struct {
    Base Asset
    Quote Asset
    Instrument Instrument
}
```

## Plan vs Execution
TradePlan = immutable approved intent.  
ExecutionState = mutable reality.

ExecutionState should represent requested, filled, protected quantities, average entry, entry state, position state, and unknown/ambiguous state.

## Single Active Lifecycle
MVP enforces at most one active entry/position lifecycle.

## Restart
Live mode reconciles local state with exchange orders, fills, balances, and protection before allowing new exposure.
