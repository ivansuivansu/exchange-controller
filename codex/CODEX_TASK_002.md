Implement Telegram approval workflow on top of the current architecture.

Requirements:

1. Add a Telegram adapter package.
   Suggested location:
   internal/telegram

2. Telegram adapter must implement PlanApprover.

3. Only configured Telegram user/chat IDs may interact with trading actions.

4. Render a TradePlan with at least:
   - instrument
   - entry price
   - quantity/notional
   - take profit
   - stop loss
   - approve-by deadline
   - entry TTL
   - plan ID
   - plan version

5. Add inline buttons:
   - APPROVE
   - EDIT
   - REJECT

6. Callback payload must identify:
   - plan ID
   - plan version
   - action

7. Validate callbacks:
   - unauthorized user -> reject
   - wrong plan version -> reject
   - expired plan -> reject
   - already decided plan -> reject
   - unknown plan -> reject

8. APPROVE:
   - returns an ApprovalDecision
   - must not directly call ExecutionEngine
   - ExecutionController remains responsible for execution

9. REJECT:
   - returns a rejection decision

10. EDIT:
   - must not mutate the original TradePlan
   - should start creation of a new version
   - MVP may support editing:
     entry price
     quantity
     take profit
     stop loss
     approval deadline
     entry TTL

11. Add basic commands:
   /start
   /status

12. /status should show:
   - current mode
   - current active lifecycle if any
   - current pending TradePlan if any

13. Continue using SimulationExecution.
    Do not implement Crypto.com.

14. Do not implement live trading.

15. Add tests for:
   - authorized approval
   - unauthorized approval
   - stale version callback
   - expired plan
   - duplicate approval
   - rejection
   - edit creates a new version
   - approval does not mutate the TradePlan

16. Keep Telegram-specific types out of domain packages.

17. Run:
   go test ./...
   go vet ./...

18. Summarize:
   - files created
   - callback format
   - any architectural deviations

19. Do not put Telegram bot token, chat ID, or user ID into source code.
Read them from configuration/environment.