# Trade Planner

Planner converts a Trade Idea into one or more concrete Trade Plans.

It owns:
- entry
- size
- TP
- SL
- approval deadline
- entry expiration
- fee/risk projections

## Capital Rule
The bot uses its dedicated subaccount.

For `BTC_USD`:
- Base = BTC
- Quote = USD

The active plan may use up to 100% of available quote balance, less a technical buffer.

The approved plan captures a concrete notional/quantity; it does not mean "spend whatever 100% happens to be later".

Future planner profiles may include Conservative, Balanced, and Aggressive.
