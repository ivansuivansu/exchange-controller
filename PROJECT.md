# Exchange Controller

## Mission
`exchange-controller` is a Telegram-first cryptocurrency trading assistant.

Core rule:
> Never open or increase a live position without explicit approval of a concrete Trade Plan in Telegram.

Protective exits already included in the approved Trade Plan may execute automatically.

## Core Flow
```text
Market
→ Signal Engine
→ Signal
→ Trade Idea Builder
→ Trade Idea
→ Trade Planner
→ Trade Plan
→ Telegram Approval
→ Execution Controller
→ Exchange / Simulation
```

## MVP
- Spot only.
- `BTC_USD` only enabled initially.
- Core must remain instrument-agnostic.
- Dedicated exchange subaccount.
- Up to 100% of available quote balance may be used for the active trade.
- Keep a technical buffer for fees/rounding/minimums.
- At most one active entry/position lifecycle.
- No averaging or pyramiding.
- Partial fills are protected incrementally.
- Three modes: `live`, `backtest`, `trainer`.

## Trade Plan
The user approves the complete immutable plan, including entry, size, TP, SL, and expiration.

Any material edit creates a new version requiring new approval.

## Partial Fill Protection
```text
Requested 0.010 BTC
Filled 0.004 → protect 0.004
Filled 0.007 → protect 0.007
Filled 0.010 → protect 0.010
```

Target invariant:
```text
ProtectedQuantity == FilledQuantity
```

`ProtectedQuantity < FilledQuantity` is a risk condition.

## Expiration
- `ApproveBy`: deadline for approval.
- `EntryExpiresAt`: deadline for an already-submitted entry to keep waiting.
- Downward movement alone does not invalidate a limit-buy plan.
- Large upward movement may make an old entry stale.
- Sophisticated TradeValidator is deferred.

## Capital Boundary
The bot trades only its dedicated subaccount.

For `BTC_USD`, the quote asset is USD. The bot may allocate up to 100% of available USD, less technical buffer.
