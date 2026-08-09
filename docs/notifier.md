# BTC Average-Price Decline Notifier

Run:

```text
NOTIFIER_ENABLED=true \
TELEGRAM_BOT_TOKEN=... \
TELEGRAM_SEND_CHAT_ID=... \
TELEGRAM_ALLOWED_USER_IDS=... \
TELEGRAM_ALLOWED_CHAT_IDS=... \
go run ./cmd/exchange-controller notify
```

This runtime is informational only. It uses Crypto.com's public `BTC_USD`
ticker and Telegram text messages. It has no Trade Idea, Trade Plan, approval,
simulation, or execution dependency and cannot place a trade.

The default configuration is:

- `NOTIFIER_ENABLED=true`
- `NOTIFIER_POLL_INTERVAL=10s`
- `NOTIFIER_WINDOW_SIZE=6` (each of the previous/current windows)
- `NOTIFIER_DROP_THRESHOLD=0.0036` (0.36%)
- `NOTIFIER_COOLDOWN=5m`

After 12 observations, the notifier compares the average of samples 1–6 with
the average of samples 7–12. The 12-sample window slides forward after every
new ticker observation. All prices, averages, and ratios use fixed-point
`Decimal` arithmetic.

An alert disarms the detector until the average change recovers to at least
half the configured decline threshold. This small hysteresis, combined with
the cooldown, prevents repeated messages when the value moves around the
alert boundary. Authorized Telegram users can inspect observations and the
last alert with `/status`.
