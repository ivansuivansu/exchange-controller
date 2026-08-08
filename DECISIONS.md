# Architecture Decision Records

## ADR-0001 — Telegram-First
**Status:** Accepted  
Live exposure requires explicit Telegram approval.

## ADR-0002 — Three Modes
**Status:** Accepted  
`live`, `backtest`, `trainer`.

## ADR-0003 — Shared Core Logic
**Status:** Accepted  
Signal, Idea, and Planner logic are shared across modes.

## ADR-0004 — Domain Pipeline
**Status:** Accepted  
`Signal → Trade Idea → Trade Planner → Trade Plan → Approval → Execution`.

## ADR-0005 — Signal Engine Does Not Produce Orders
**Status:** Accepted

## ADR-0006 — Planner Owns Entry/Risk Construction
**Status:** Accepted  
Planner owns entry, size, TP, SL, expiration, and projections.

## ADR-0007 — Trade Plan Is Approval Unit
**Status:** Accepted

## ADR-0008 — Approval Is Version-Specific
**Status:** Accepted

## ADR-0009 — Protective Exits May Execute Automatically
**Status:** Accepted

## ADR-0010 — Prefer Exchange-Side Protection
**Status:** Accepted

## ADR-0011 — Incremental Protection for Partial Fills
**Status:** Accepted  
Every filled portion should be protected as soon as practical.

## ADR-0012 — Plan and Execution State Are Separate
**Status:** Accepted  
TradePlan is immutable intent; fills/orders belong to ExecutionState.

## ADR-0013 — Separate Plan and Entry Expiration
**Status:** Accepted  
Use `ApproveBy` and `EntryExpiresAt`.

## ADR-0014 — Downward Movement Alone Does Not Invalidate Entry
**Status:** Accepted

## ADR-0015 — Basic Validation Only in MVP
**Status:** Accepted  
No dedicated TradeValidator yet.

## ADR-0016 — Dedicated Subaccount
**Status:** Accepted  
The subaccount is the capital and reconciliation boundary.

## ADR-0017 — 100% Available Quote-Balance Allocation
**Status:** Accepted  
Use up to 100% of available quote balance, less technical buffer.

## ADR-0018 — One Active Position/Lifecycle
**Status:** Accepted  
No second executable live plan until the active lifecycle closes.

## ADR-0019 — BTC_USD Only, Instrument-Agnostic Core
**Status:** Accepted

## ADR-0020 — No Averaging or Pyramiding in MVP
**Status:** Accepted

## ADR-0021 — Unknown Exchange State Is First-Class
**Status:** Accepted  
Ambiguous timeout must not be blindly retried.

## ADR-0022 — Reconcile Before Normal Live Operation
**Status:** Accepted

## ADR-0023 — Initial Signal Candidate
**Status:** Proposed  
Recent local high + drawdown + local low + recovery.
