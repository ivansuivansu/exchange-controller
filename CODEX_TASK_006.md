Implement CODEX_TASK_006: first real BTC strategy research runner.

GOAL

Make the existing backtester convenient to run against real historical
BTC_USD candles over a user-selected date range and export detailed results.

Do NOT optimize strategy parameters yet.

1. Historical dataset

Add a command that can obtain historical BTC_USD M1 candles for a specified:

- start date/time
- end date/time

Use the existing Crypto.com public market-data adapter where possible.

Crypto.com API pagination/limits must be handled correctly.

Downloaded candles must be:

- chronological
- deduplicated
- validated
- completed candles only

2. Dataset caching

Allow downloaded historical candles to be saved locally.

Preferred simple format:

CSV

Fields:

timestamp
open
high
low
close
volume

Allow the backtester to run from the local dataset without downloading
the same history again.

Do not add a database.

3. Backtest CLI

Support a practical command such as:

go run ./cmd/exchange-controller backtest \
  --file data/btc_usd_m1.csv

and/or:

go run ./cmd/exchange-controller backtest \
  --from ... \
  --to ...

Exact CLI syntax may be refined.

4. Explicit strategy configuration

Print the complete strategy/planner configuration at the beginning
of every run.

This includes at least:

- timeframe
- lookback
- drawdown threshold
- recovery threshold
- cooldown/reset
- entry mode
- entry offset
- TP mode
- retracement target
- SL offset
- minimum R/R
- entry TTL
- technical reserve
- fee rate
- ambiguity policy

Never produce a backtest report without also making the tested
configuration identifiable.

5. Initial research preset

Add one clearly labeled PLACEHOLDER research preset.

Use:

Timeframe: M1
Lookback: 60 candles
Drawdown threshold: 0.01
Recovery threshold: 0.005

Entry mode:
RECOVERY_CLOSE

TP mode:
PREVIOUS_HIGH

SL offset:
0.005 below LocalLow

Minimum R/R:
1.0

Starting capital:
10000 USD

Ambiguity:
CONSERVATIVE

Do not claim these parameters are good or optimized.

Remember:
0.01 = 1%
0.005 = 0.5%

6. Fee configuration

Fee rate must remain configurable.

Do not silently invent or hardcode a claim about the user's actual
Crypto.com fee tier.

The CLI/report must clearly print the fee rate used in the simulation.

7. Detailed trade export

Export every completed trade to CSV.

Include at least:

plan_id
idea_id
signal_time
recent_high
local_low
recovery_price
entry_submitted_at
entry_filled_at
planned_entry
actual_entry
take_profit
stop_loss
exit_at
exit_reason
exit_price
quantity
entry_fee
exit_fee
gross_pnl
net_pnl
return_percent
capital_before
capital_after
ambiguous

Also export unfilled/expired entries where practical.

8. Summary report

Print and optionally save:

Starting capital
Ending capital
Net PnL
Total return
Signals
Plans
Planner rejections
Filled entries
Expired entries
Wins
Losses
Win rate
Average win
Average loss
Profit factor
Maximum drawdown
Total fees
Ambiguous trades

9. Rejection report

Show planner rejection counts grouped by reason.

Example:

minimum_risk_reward: 42
invalid_take_profit: 8
insufficient_capital: 0

10. Monthly breakdown

Produce a simple monthly breakdown:

month
trades
wins
losses
net_pnl
return_percent

This is important so a profitable total result cannot hide that the
strategy only worked during one short market regime.

11. Determinism

Running the same dataset and configuration twice must produce identical
results.

Add a test for this if not already covered.

12. Dataset identity

Report:

- first candle timestamp
- last candle timestamp
- candle count
- timeframe
- instrument

This allows us to know exactly what was tested.

13. No parameter optimization

DO NOT implement:

- grid search
- automatic parameter tuning
- "best settings"
- genetic algorithms
- ML
- walk-forward optimization

We first want to inspect one understandable baseline strategy.

14. Tests

Add tests for:

- CSV historical loader
- CSV writer/reader roundtrip
- duplicate removal
- chronological ordering
- requested date range handling
- trade CSV export
- monthly aggregation
- configuration printed with report
- deterministic repeated run

15. Verification

Run:

go test ./...
go vet ./...

Then perform one small sample backtest if fixture/sample data is available.

Summarize:

- CLI usage
- historical download behavior
- cache format
- research preset
- report/export files
- tests
- architectural deviations