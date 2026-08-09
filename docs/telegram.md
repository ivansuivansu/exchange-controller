# Telegram UX

User approves the complete Trade Plan.

Example:
```text
🔴 LIVE

BTC_USD Trade Plan
LIMIT BUY @ $64,100
Planned capital: $4,985
TP: $65,800
SL: $63,500

[ APPROVE ] [ EDIT ] [ REJECT ]
```

Partial fill:
```text
Requested: 0.0780 BTC
Filled:    0.0310 BTC
Protected: 0.0310 BTC
Remaining: 0.0470 BTC
```

Risk warning:
```text
⚠️ POSITION PARTIALLY UNPROTECTED
Filled:      0.0310 BTC
Protected:   0.0200 BTC
Unprotected: 0.0110 BTC
```

## MVP Approval Workflow

Only one Trade Plan may be pending approval at a time. A second plan is not
presented until the pending plan is approved, rejected, or replaced through
EDIT. EDIT creates a new immutable version and immediately presents that new
version with fresh approval buttons.

Callback authenticity is enforced in the MVP through the configured user/chat
allowlist and server-side plan/version/decision checks. Callback signing or
HMAC is deferred.

Telegram-enabled startup reads and validates:

- `TELEGRAM_BOT_TOKEN`
- `TELEGRAM_SEND_CHAT_ID`
- `TELEGRAM_ALLOWED_USER_IDS` (comma-separated)
- `TELEGRAM_ALLOWED_CHAT_IDS` (comma-separated)
