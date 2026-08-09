Perform a small hardening pass on Task 002 before market-data work.

1. Define and implement the orchestration after EDIT:
   an edited TradePlan version must be presented to the user for a new
   approval. Do not leave the new version registered but invisible.

2. Validate Telegram configuration at startup.
   In Telegram-enabled operation:
   - bot token must not be empty
   - SendChatID must be configured
   - authorization configuration must not result in a bot that can never
     authorize anyone
   Keep secrets in environment/config only.

3. Document/test that the MVP intentionally supports only one pending
   TradePlan at a time.

4. Do not add callback signing/HMAC yet.
   Existing server-side plan/version/authorization checks are sufficient
   for the MVP.

5. Check go.mod's Go version against the actual project toolchain.
   Do not require a newer Go version than necessary.

6. Add/update tests for the above.

Do not implement Crypto.com or market data yet.

Run:
go test ./...
go vet ./...