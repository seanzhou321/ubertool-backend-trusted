# Rental E2E Test — Session Notes

## Current Status

| Task | Status |
|------|--------|
| Proto field `RentalRequest.ChargeBillsplit` (field 29) | ✅ Added, regenerated |
| Mapper: `ChargeBillsplit: r.ChargeBillsplit` | ✅ Added |
| DB column `charge_billsplit BOOLEAN NOT NULL DEFAULT TRUE` | ✅ Fixed |
| COALESCE default → `true` in all 4 repo queries | ✅ Fixed |
| Double-balance bug (DB trigger + service both updating balance) | ✅ Fixed — service no longer manually updates balance |
| DB trigger extended to set `last_balance_updated_on = CURRENT_DATE` | ✅ Done |
| Unit tests after balance-update removal | ✅ Passing |
| E2E `Full_Rental_Lifecycle` (chargeBillsplit=true) | ✅ Passing |
| E2E `Full_Rental_Lifecycle_with_charge_billsplit=false` | ✅ Fixed — polling helper in `assertDirectSettlementReminders` |

---

## Architecture Overview

### Stack
- **Language**: Go 1.24.x  
- **Transport**: gRPC (`protoc` generated, `api/gen/v1/`)  
- **DB**: PostgreSQL on `localhost:5454` (test), schema in `podman/trusted-group/postgres/ubertool_schema_trusted.sql`  
- **E2E**: Tests hit a **separately running server** at port 50052 via gRPC

### Key Files

| File | Role |
|------|------|
| `internal/service/rental.go` | Core business logic — `CompleteRental`, settlement helpers |
| `internal/service/notification.go` | `Dispatch` = DB INSERT then optional FCM push |
| `internal/repository/postgres/rental.go` | 4 SELECT queries with `COALESCE(charge_billsplit, true)` |
| `internal/repository/postgres/notification.go` | `Create` = INSERT INTO notifications RETURNING id |
| `api/proto/.../rental_service.proto` | `RentalRequest.charge_billsplit` = field 29 |
| `api/gen/v1/rental_service.pb.go` | Generated — do not edit manually |
| `internal/api/grpc/mapper.go` | `MapDomainRentalToProtoWithNames` includes `ChargeBillsplit` |
| `tests/e2e/rental_test.go` | E2E test suite (modularized — see helpers below) |
| `tests/e2e/rental_steps_test.go` | Step helpers extracted from test cases |

---

## The Double-Balance Bug (RESOLVED)

**Root cause**: A DB trigger `update_user_balance()` fires `AFTER INSERT ON ledger_transactions FOR EACH ROW` and does `UPDATE users_orgs SET balance_cents = balance_cents + NEW.amount`. The service ALSO manually fetched and updated balance. Every settlement applied 2×.

**Fix**: Removed manual balance update from `applyOwnerSettlement` and `applyRenterSettlement`. Those functions now only call `ledgerRepo.CreateTransaction`. The trigger is the sole authority on balance.

```sql
-- Trigger (in ubertool_schema_trusted.sql)
CREATE OR REPLACE FUNCTION update_user_balance() RETURNS TRIGGER AS $$
BEGIN
    UPDATE users_orgs
    SET balance_cents = balance_cents + NEW.amount,
        last_balance_updated_on = CURRENT_DATE
    WHERE user_id = NEW.user_id AND org_id = NEW.org_id;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- Fires: AFTER INSERT ON ledger_transactions FOR EACH ROW
```

---

## The chargeBillsplit=false Flow

When `CompleteRental` is called with `chargeBillsplit=false`:

1. `applyOwnerSettlement` → **no-op**, returns `ownerLedgerID=0`
2. `applyRenterSettlement` → **no-op**, returns `renterLedgerID=0`
3. **No ledger transactions created** → no balance changes
4. `go s.dispatchSettlementNotifications(...)` — reminder text appended to messages:
   - Owner: `"Your rental has been credited N cents. Reminder: The rental payment...settled directly between you and the renter..."`
   - Renter: `"Your rental has been debited N cents. Reminder: The rental payment...settled directly between you and the owner..."`

---

## Active Bug: Settlement Notifications Not Found

### Failing assertions (rental_test.go, `chargeBillsplit=false` sub-test)

```go
// Both return count=0 after 200ms sleep
SELECT COUNT(*) FROM notifications
WHERE user_id=$ownerID AND org_id=$orgID
  AND message LIKE '%settled directly between you and the renter%'

SELECT COUNT(*) FROM notifications
WHERE user_id=$renterID AND org_id=$orgID
  AND message LIKE '%settled directly between you and the owner%'
```

Note: The `ownerNotifCount >= 1` check PASSES because earlier RPC steps
(CreateRentalRequest, ApproveRentalRequest, FinalizeRentalRequest) already inserted
notifications for those users.

### Root cause

`dispatchSettlementNotifications` runs as a goroutine (`go s.dispatchSettlementNotifications(...)`). Notifications are **intentionally fire-and-forget** — the gRPC handler must return immediately without waiting for notification delivery. The test was using a fixed 200 ms sleep, which was not long enough on a loaded Windows machine.

### Applied fix

Replaced the fixed `time.Sleep(200 ms)` + direct assertion with a **polling helper** inside `assertDirectSettlementReminders`. The helper polls the `notifications` table every 100 ms until the expected row appears, with a 5-second hard deadline. This means:

- The test completes quickly when the goroutine finishes fast (typically < 50 ms)
- On a loaded machine the test still has 5 seconds of tolerance
- `dispatchSettlementNotifications` remains a goroutine — the gRPC response latency is unaffected

```go
// rental_steps_test.go — pollDB helper
func pollDB(timeout, pollInterval time.Duration, f func() bool) bool {
    deadline := time.Now().Add(timeout)
    for {
        if f() { return true }
        if time.Now().Add(pollInterval).After(deadline) { return false }
        time.Sleep(pollInterval)
    }
}
```

**Note:** Test users have no FCM tokens. `SendToUser` returns immediately after a fast "no tokens found" DB query — it does NOT wait for `fcmSendTimeout` (10 s). The only delay before the notification row appears in the DB is goroutine scheduling + a single DB INSERT, both well under 1 second.

### What was ruled out
- `context.WithoutCancel` is valid — inherits values, ignores Done/Deadline
- `noteSvc` is not nil (wired properly in `cmd/server/main.go`)
- Firebase IS initialized (real key in `config/firebase-admin-key.json`) — but test users have no FCM tokens so `SendToUser` returns immediately after querying
- No nil pointer risks in `dispatchSettlementNotifications` — all fields guarded
- No unique constraint on `notifications` table
- Sub-tests use their own `orgID`/`ownerID`/`renterID` (no cross-contamination)

---

## Context: Proto Field Mapping

```
CompleteRentalRequest.charge_billsplit (field 5)
    → rentalSvc.CompleteRental(..., chargeBillsplit bool)
        → rt.ChargeBillsplit = chargeBillsplit
        → rentalRepo.Update(ctx, rt)      -- persists to DB
        → applyOwnerSettlement(...)       -- skipped when false
        → applyRenterSettlement(...)      -- skipped when false
        → go dispatchSettlementNotifications(...)

RentalRequest.charge_billsplit (field 29)  ← ADDED in this session
    ← MapDomainRentalToProtoWithNames → ChargeBillsplit: r.ChargeBillsplit
```

---

## Test Structure (after modularization)

```
tests/e2e/
  rental_test.go           — Top-level test function + sub-test registration
  rental_steps_test.go     — Step helpers: runRentalLifecycle(), runNoBillsplitLifecycle(), etc.
  helpers.go               — DB helpers, gRPC client, context builders
```

### Step helper signatures (in rental_steps_test.go)

```go
// Core rental workflow sub-steps
func setupRentalTestEnv(t, db, email suffix, initialBalance) (orgID, ownerID, renterID, toolID int32)
func doCreateRentalRequest(t, client, renterID, toolID, orgID, startDate, endDate) (rentalID int32)
func doApproveRentalRequest(t, client, ownerID, rentalID, pickupNote) 
func doFinalizeRentalRequest(t, client, renterID, rentalID)
func doActivateRental(t, client, ownerID, rentalID)
func doCompleteRental(t, client, ownerID, rentalID, chargeBillsplit bool) *pb.RentalRequest
func assertSettlementNotifications(t, db, ownerID, renterID, orgID, chargeBillsplit bool)
func assertNotificationReminderText(t, db, ownerID, renterID, orgID)
func assertBalanceAndLedger(t, db, ownerID, renterID, orgID, chargeBillsplit bool, expectedSettlement int32)
```
