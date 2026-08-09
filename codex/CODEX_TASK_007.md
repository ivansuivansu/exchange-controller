Implement a Telegram BTC average-price decline notifier.

GOAL

Poll BTC_USD every 10 seconds and notify the authorized Telegram user
when the short-term average price has fallen by at least 0.36%.

This feature is informational only.
It must never create or execute a trade.

1. Polling

Use the existing Crypto.com public ticker adapter.

Default polling interval:

10 seconds

2. Rolling observations

Store the latest 12 price observations.

At a 10-second polling interval this represents approximately 2 minutes.

3. Average calculation

Split the 12 observations into two consecutive windows:

Previous window:
observations 1-6

Current window:
observations 7-12

Calculate:

previousAverage
currentAverage

Use Decimal/fixed-point arithmetic only.

4. Decline calculation

Calculate:

change = (currentAverage - previousAverage) / previousAverage

Trigger an alert when:

change <= -0.0036

Remember:

0.0036 = 0.36%

Do not use float64.

5. Sliding behavior

Evaluate every 10 seconds.

After every new observation the 12-value window moves forward by one
observation.

Do NOT wait for another completely independent two-minute block.

6. Alert cooldown

Default:

5 minutes

Do not repeatedly notify every 10 seconds during the same decline.

7. Reset behavior

The alert state may reset after the average-price decline no longer meets
the configured threshold.

Avoid notification spam caused by movement immediately around -0.36%.

8. Telegram notification

Example:

📉 BTC price decline detected

Current:       $64,820
Previous avg:  $65,080
Current avg:   $64,840

Average change: -0.37%
Threshold:      -0.36%
Window:         ~2 minutes

INFORMATION ONLY
No trade was placed.

9. Configuration

Add:

NOTIFIER_ENABLED=true
NOTIFIER_POLL_INTERVAL=10s
NOTIFIER_WINDOW_SIZE=6
NOTIFIER_DROP_THRESHOLD=0.0036
NOTIFIER_COOLDOWN=5m

WINDOW_SIZE means the size of EACH averaging window.

Total observations required:

2 * WINDOW_SIZE

10. /status

Show:

Notifier: ON
Instrument: BTC_USD
Poll interval: 10s
Average window: 6 samples (~1 min)
Comparison period: ~2 min
Drop threshold: 0.36%
Cooldown: 5m
Current price: ...
Previous average: ...
Current average: ...
Current change: ...
Last alert: ...

11. Tests

Test:

- stable prices -> no alert
- rising average -> no alert
- decline of 0.35% -> no alert
- decline exactly 0.36% -> alert
- decline greater than 0.36% -> alert
- sliding-window calculation
- cooldown
- reset after recovery
- Decimal average calculation
- human percentage rendering
- no duplicate alert every 10 seconds

Do not call real Telegram/Crypto.com in unit tests.

12. Runtime

Provide:

go run ./cmd/exchange-controller notify

13. Non-goals

Do NOT:
- create TradeIdeas
- create TradePlans
- simulate trades
- place real trades
- optimize the threshold

Run:

go test ./...
go vet ./...