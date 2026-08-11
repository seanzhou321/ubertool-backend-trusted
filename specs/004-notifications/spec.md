# Feature Specification: Notifications (As-Built)

**Feature Branch**: `005-notifications`

**Created**: 2026-07-22

**Status**: Draft

**Input**: Retrofit specification for the existing, already-implemented and deployed
Notifications feature (in-app notifications + FCM push delivery). Per project constitution
Principle I ("Reconcile Discrepancies Among Spec, RTM, and Code") and Principle III (Push Notification Pattern), this document
describes verified current behavior of `internal/service/notification.go`,
`internal/service/push_notification.go`, `internal/api/grpc/notification.go`, the
`notifications`/`fcm_tokens` tables in
`podman/trusted-group/postgres/ubertool_schema_trusted.sql`, and
`api/proto/ubertool_trusted_backend/v1/notification_service.proto` — cross-checked against
`docs/design/grpc_api_business_logic.md`'s "Notifications" section and "Push Notification
Pattern" architectural note, and `docs/design/push_notification.md`. It is **not** a
proposal for new behavior; every gap found is called out in "Known Discrepancies" below.

This spec covers the `NotificationService` RPC surface and the FCM push delivery engine it
shares with every other domain (Bill Split, Organizations, Rentals, Admin) via
`NotificationService.Dispatch`/`DispatchSilent` and `PushNotificationService.SendToUser`/
`SendMulticastToUsers`. It does not re-specify *when* each domain triggers a notification —
that business logic lives in each domain's own spec (e.g.
`specs/004-organizations-administration/spec.md`'s threshold-broadcast use of
`SendMulticastToUsers`).

## User Scenarios & Testing *(mandatory)*

### User Story 1 - In-App Notification Retrieval & Read State (Priority: P1)

A user pages through their notification history and marks individual notifications read.

**Why this priority**: The only user-facing read path in this domain; every other RPC here
is either write-only (device sync, event reporting) or infrastructure (push delivery).

**Independent Test**: Seed several notifications for a user; call `GetNotifications` with
various `limit`/`offset` combinations; call `MarkNotificationRead` and confirm `read_at` is
stamped exactly once even if called twice.

**Acceptance Scenarios**:

1. **Given** an authenticated user, **When** `GetNotifications` is called with `limit` and
   `offset`, **Then** the handler defaults `limit` to 10 if `limit <= 0`, converts to the
   service's page-based signature via `page = (offset / limit) + 1`, and returns that
   user's notifications plus a `total_count`.
2. **Given** a notification belonging to the caller with `read_at IS NULL`, **When**
   `MarkNotificationRead` is called, **Then** `read_at` is set to `NOW()` (first-write-wins
   — a second call is a no-op, not an error).
3. **Given** a `notification_id` that does not exist, or exists but belongs to a different
   user, **When** `MarkNotificationRead` is called, **Then** it is rejected ("notification
   not found or access denied") — ownership is enforced by the `WHERE id = $1 AND user_id =
   $2` update itself returning zero affected rows.

---

### User Story 2 - Push Notification Delivery Pipeline (Priority: P1)

Every domain event that should notify a user inserts a `notifications` row via `Dispatch`,
which then fans out to that user's active devices through a bounded worker pool with
per-message retry and permanent-failure (obsolete token) handling.

**Why this priority**: This is the shared infrastructure every other domain's
notify-worthy event (per Constitution Principle III) depends on; a defect here silently
degrades push delivery for the entire system, not just one domain.

**Independent Test**: Call `Dispatch` for a user with two active FCM tokens and confirm both
receive an enqueued send; simulate an `UNREGISTERED` FCM error for one token and confirm
only that token is marked `OBSOLETE` while the other is unaffected.

**Acceptance Scenarios**:

1. **Given** `Dispatch` is called with a notification, **When** the `notifications` INSERT
   succeeds and a push service is configured, **Then** a push job is enqueued for every
   `ACTIVE` FCM token belonging to that user, embedding the new `notification_id` in the
   FCM `data` payload alongside `title`/`body`; a *data-only* message is sent (no FCM
   `notification` field), consistent with `docs/design/push_notification.md`'s "data-only
   messages are mandatory" guidance.
2. **Given** `DispatchSilent` is called instead, **When** it completes, **Then** the
   `notifications` row is created but no push job is enqueued — used when the caller
   handles push delivery itself via `SendMulticastToUsers` (e.g. the Organizations
   threshold-update broadcast) to avoid a duplicate per-user send.
3. **Given** a token has `status = 'TESTING'`, **When** a push job for it is processed,
   **Then** it is routed through Firebase's dry-run (validate-only) send path — the
   message is never actually delivered, and validation failures on that path never mark
   the token obsolete (fake tokens are expected to fail validation).
4. **Given** a real (non-`TESTING`) send fails with `messaging.IsUnregistered` or
   `messaging.IsInvalidArgument`, **When** the worker processes that result, **Then** the
   token is marked `OBSOLETE` in `fcm_tokens` and no retry is scheduled.
5. **Given** a real send fails with any other error (transient), **When** the worker
   processes that result, **Then** a retry is scheduled after the next back-off delay
   (1 min, then 2 min, then 4 min) up to 3 attempts total, after which it is logged and
   dropped.
6. **Given** the FCM job queue (capacity 256) is full, **When** a new push job is enqueued
   (from `Dispatch`, a retry, or `SendMulticastToUsers`), **Then** the job is dropped with a
   warning log rather than blocking the caller — `Dispatch`/`SendToUser` never block on
   push delivery.
7. **Given** the channel is `rental_request` (`domain.ChannelRentalRequest`), **When** a
   push is sent, **Then** Android delivery priority is `"high"` (wakes the device from
   Doze); every other channel uses `"normal"` priority.

---

### User Story 3 - Device Token Registration (Priority: P2)

The mobile client registers or refreshes its FCM token on app start and whenever Android
issues a new one.

**Why this priority**: A prerequisite for push delivery (User Story 2) but a simple
upsert with no business logic of its own.

**Independent Test**: Call `SyncDeviceToken` for a new token and confirm an `ACTIVE`
`fcm_tokens` row is created; call it again with the same token and a different user and
confirm the row's `user_id` is reassigned (covers the re-login-on-same-device case).

**Acceptance Scenarios**:

1. **Given** an authenticated user and an FCM token not yet in `fcm_tokens`, **When**
   `SyncDeviceToken` is called, **Then** a new row is inserted with `status = 'ACTIVE'`.
2. **Given** the same FCM token already exists (matched by the table's `UNIQUE(fcm_token)`
   constraint), **When** `SyncDeviceToken` is called by a different user, **Then** the
   existing row's `user_id`, `device_info`, and `status` are updated to the new caller —
   handling the case where a user logs out and a different user logs in on the same
   physical device.

---

### User Story 4 - Delivery/Click Event Reporting (Priority: P2)

The mobile client reports back when a push notification was delivered to the device and
when the user tapped it, so the backend can track engagement.

**Why this priority**: Telemetry that does not gate any other functional flow.

**Independent Test**: Call `ReportMessageEvent` with `event_type = "DELIVERED"` for a real
notification and confirm `delivered_at` is stamped once (first-write-wins); repeat for
`"CLICKED"`.

**Acceptance Scenarios**:

1. **Given** `event_type = "DELIVERED"`, **When** `ReportMessageEvent` is called, **Then**
   `notifications.delivered_at` is set via `COALESCE(delivered_at, $event_time)` — a second
   report does not overwrite the original timestamp.
2. **Given** `event_type = "CLICKED"`, **When** `ReportMessageEvent` is called, **Then**
   `notifications.clicked_at` is set the same way.
3. **Given** an `event_type` other than `DELIVERED`/`CLICKED`, **When**
   `ReportMessageEvent` is called, **Then** it is rejected ("unknown event_type").
4. **Given** a `notification_id` that does not exist or does not belong to the caller,
   **When** `ReportMessageEvent` is called, **Then** — **as-built** — the call still
   returns success (see Known Discrepancy 2); no error is surfaced and no row is changed.

---

### User Story 5 - Broadcast Push to Many Users (Priority: P2)

A domain event affecting many members at once (e.g. an org-wide threshold change) sends one
push payload to all of them via FCM multicast rather than one `SendToUser` call per member.

**Why this priority**: An optimization path used by exactly one caller today
(`organizationService.broadcastThresholdUpdate`); important to specify precisely because it
has zero automated test coverage (see Current Test Coverage Baseline) despite being live in
production.

**Independent Test**: Call `SendMulticastToUsers` with a list of user IDs spanning more
than 500 active tokens combined, and confirm the tokens are batched into multiple
`SendEachForMulticast` calls of at most 500 tokens each.

**Acceptance Scenarios**:

1. **Given** a list of `userIDs`, **When** `SendMulticastToUsers` is called, **Then** it
   returns immediately (token lookup and sending happen in a background goroutine) and
   fetches all `ACTIVE` tokens across all given users in one query.
2. **Given** more than 500 combined tokens, **When** they are sent, **Then** they are split
   into batches of at most 500 (Firebase's multicast limit) and sent via successive
   `SendEachForMulticast` calls.
3. **Given** a token in a multicast batch fails with `messaging.IsUnregistered`, **When**
   the batch response is processed, **Then** that token is marked `OBSOLETE` — **unlike**
   the per-user `SendToUser` path (User Story 2 Scenario 5), a multicast batch failure is
   **not** retried; there is no back-off/retry mechanism for multicast sends.

---

### Edge Cases

- `SendMulticastToUsers`'s per-batch failures are logged and skipped, not retried — a
  transient failure on a multicast batch permanently loses that push for that batch's
  recipients, unlike single-user pushes which get 3 retries.
- `Dispatch`'s push step is fire-and-forget from the caller's perspective: the `error`
  returned by `s.pushSvc.SendToUser(...)` inside `Dispatch` is explicitly discarded
  (`//nolint:errcheck`) — a push enqueue failure never surfaces as a `Dispatch` error, only
  as a log line.
- A `fcm_tokens` row's `device_info` is fully replaced (not merged) on every
  `SyncDeviceToken` call for that token.

## Known Discrepancies *(code vs. documentation, verified against source)*

1. **`GetNotifications` silently rounds non-page-aligned `offset` values down to the
   nearest page boundary.** The proto (`GetNotificationsRequest`) exposes `limit`/`offset`
   as if arbitrary-offset pagination were supported, and the handler
   (`internal/api/grpc/notification.go`) converts to the service's `page`/`pageSize`
   signature via integer division: `page = (offset / limit) + 1`. This round-trip is only
   correct when `offset` is an exact multiple of `limit`. For example, `limit=10,
   offset=5` computes `page=1` — identical to `offset=0` — so the "5" is silently dropped
   rather than skipping 5 records. The only existing pagination test
   (`tests/e2e/notification_test.go`, "GetNotifications with Pagination") only exercises
   `offset=0` and `offset=10` against `limit=10`, both exact multiples, so it does not
   catch this.
2. **`ReportMessageEvent` does not verify notification ownership for `DELIVERED`/`CLICKED`
   events, contrary to documentation.** `grpc_api_business_logic.md`'s "Report Message
   Event" step 3 states: "Verify `notification_id` exists in `notifications` and belongs to
   `user_id`." **As-built**, `notificationRepository.MarkDelivered`/`MarkClicked`
   (`internal/repository/postgres/notification.go`) run an `UPDATE ... WHERE id = $1 AND
   user_id = $2` but — unlike the structurally identical `MarkAsRead`, which checks
   `RowsAffected() == 0` and returns "notification not found or access denied" — neither
   checks the affected row count. A `ReportMessageEvent` call for a nonexistent or
   not-owned `notification_id` silently returns success with no error and no state change.
3. **`docs/design/push_notification.md` describes a schema and table design that was never
   built.** It documents a `user_devices` table (PK `fcm_token`, `user_id` as `UUID`) and a
   separate `notification_logs` table with a `status` ENUM (`SENT`/`DELIVERED`/`CLICKED`).
   The actual schema uses `fcm_tokens` (`user_id INTEGER`, `UNIQUE(fcm_token)`, not a PK)
   and stores delivery/click timestamps directly on the `notifications` table itself — no
   separate log table, no status enum. This document predates the implemented design and
   should not be used as a reference for the current schema.
4. **`grpc_api_business_logic.md`'s "Push Notification Pattern" note describes a much
   simpler mechanism than what is actually implemented.** It says push sends are "goroutine
   asynchronously (goroutine with a detached context and timeout)" per notification.
   **As-built**, delivery instead runs through a shared, bounded 5-worker pool
   (`internal/service/push_notification.go`) that micro-batches jobs (up to 500 per
   `SendEach` call, 100ms collection window), retries transient failures with back-off (1m/
   2m/4m, 3 attempts), routes `TESTING`-status tokens through Firebase's dry-run API
   instead of real delivery, and supports a separate FCM-multicast path for broadcasts.
   This is not a defect — the implementation is strictly more capable than the doc
   describes — but anyone relying on the doc alone would significantly underestimate the
   system's actual behavior (e.g. not realize retries happen at all).

## Current Test Coverage Baseline *(informational — grounds the next /speckit-tasks pass, not a requirement)*

Verified by inspection of `tests/unit/push_notification_service_test.go` (14 tests),
`tests/e2e/push_notification_test.go`, and `tests/e2e/notification_test.go`:

**Covered, and covered well**: the single-user push worker pipeline —
`SendToUser` with zero/one/many active tokens, payload contents (notification ID embedded),
retry back-off ordering, all-retries-exhausted logging, unregistered-token → obsolete
marking (including when the obsolete-mark itself fails), and transient-then-permanent
error sequencing. This is one of the better-tested subsystems in the codebase. E2E:
`GetNotifications` (including the offset=0/offset=10 pagination case), full push
notification round-trip.

**Not covered anywhere**:

- The non-page-aligned `offset` bug in `GetNotifications` (Known Discrepancy 1).
- `ReportMessageEvent`'s missing ownership check for `DELIVERED`/`CLICKED` (Known
  Discrepancy 2) — no test calls it with another user's `notification_id`.
- `SendMulticastToUsers` — **zero** automated tests at any tier, despite being used in
  production by the Organizations threshold-broadcast flow (>500-token batching, per-batch
  `OBSOLETE` marking, and the no-retry-on-transient-failure behavior in Edge Cases are all
  unverified).
- `notificationService.MarkAsRead`'s "not found or access denied" rejection path at the
  unit-test tier (only reachable indirectly via e2e today).
- `SyncDeviceToken`'s re-registration-by-a-different-user path (User Story 3 Scenario 2).
- `TESTING`-status token handling (dry-run routing) — the unit suite exercises the
  production `SendEach` path but not the `SendEachDryRun` path directly.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: `GetNotifications` MUST return the caller's own notifications only, with a
  `total_count`, and MUST support `limit`/`offset` semantics as documented in the proto —
  as-built this is violated for non-page-aligned offsets (Known Discrepancy 1).
- **FR-002**: `MarkNotificationRead` MUST be idempotent (first-write-wins on `read_at`) and
  MUST reject a `notification_id` that does not exist or does not belong to the caller.
- **FR-003**: `Dispatch` MUST persist the notification before attempting any push send, and
  MUST NOT let a push-delivery failure surface as a `Dispatch` error.
- **FR-004**: Every push send MUST embed the originating `notification_id` in the FCM data
  payload so the client can round-trip it via `ReportMessageEvent`.
- **FR-005**: A token whose send fails with `Unregistered` or `InvalidArgument` MUST be
  marked `OBSOLETE` and MUST NOT be retried; any other failure MUST be retried up to 3
  times with back-off before being dropped.
- **FR-006**: `SyncDeviceToken` MUST upsert on the FCM token's own uniqueness, reassigning
  `user_id` when the same token re-registers under a different user.
- **FR-007**: `ReportMessageEvent` MUST reject an `event_type` outside `{DELIVERED,
  CLICKED}`. As-built, it MUST NOT be assumed to reject an invalid/foreign
  `notification_id` (Known Discrepancy 2) — this requirement is the target correctness bar
  for a follow-up task, not current behavior.
- **FR-008**: `SendMulticastToUsers` MUST batch tokens into groups of at most 500 per
  Firebase multicast call and MUST mark `OBSOLETE` any token reported unregistered in a
  batch response.

### Key Entities

- **Notification**: `notifications` table — `user_id`, `org_id`, `title`, `message`,
  `attributes` (JSONB metadata), `delivered_at`/`clicked_at`/`read_at` (each first-write-wins),
  `created_at`.
- **FcmToken**: `fcm_tokens` table — one row per device registration, globally unique on
  `fcm_token`, `status` (`ACTIVE`/`OBSOLETE`/`TESTING`), `device_info` (JSONB).
- **fcmJob** (in-process only, not persisted): a queued unit of push work — token, message,
  attempt count, dry-run flag — consumed by the fixed-size worker pool.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: The `GetNotifications` offset-rounding bug (Known Discrepancy 1) is either
  fixed (service accepts a true offset rather than round-tripping through `page`) or the
  proto/doc is corrected to describe page-based pagination explicitly — it does not remain
  a silent mismatch between what the API signature implies and what it does.
- **SC-002**: `ReportMessageEvent` either gains the same ownership check `MarkAsRead`
  already has, or the spec/doc is updated to state explicitly that delivery/click reporting
  is unauthenticated-relative-to-ownership by design — it does not remain an undocumented
  inconsistency between two structurally identical methods.
- **SC-003**: `SendMulticastToUsers` has automated test coverage for its batching threshold
  (>500 tokens), its obsolete-marking behavior, and its no-retry-on-transient-failure
  behavior — closing the single largest test gap in this domain.
- **SC-004**: A developer reading only this spec can correctly describe the actual push
  delivery mechanism (worker pool, batching, retries, dry-run tokens) without needing to
  read `internal/service/push_notification.go` — closing the gap left by Known Discrepancy
  4.

## Assumptions

- Which domain events trigger a `Dispatch`/`DispatchSilent` call, and their in-app
  title/message/attribute content, are specified per-domain (Bill Split, Organizations,
  Rentals, Admin), not here — this spec covers only the shared delivery mechanism.
- `docs/design/push_notification.md` and `docs/design/frontend-push-notification/` are
  historical design artifacts; this spec supersedes the backend-schema portions of the
  former (Known Discrepancy 3) but does not evaluate the frontend-facing content of either.
- Fixing Known Discrepancies 1 and 2 are small, low-risk changes (an offset-vs-page fix and
  an added row-count check, respectively) consistent with patterns already used elsewhere
  in the same files; this spec documents them as gaps but does not itself apply the fix.
