# Feature Specification: Bill Split (As-Built)

**Feature Branch**: `001-bill-split`

**Created**: 2026-07-22

**Status**: Draft

**Input**: Retrofit specification for the existing, already-implemented and deployed Bill
Split feature. Per project constitution Principle I ("Reconcile Discrepancies Among Spec, RTM, and Code"), this document
describes verified current behavior of `internal/service/bill_split.go`,
`internal/api/grpc/bill_split.go`, `internal/jobs/billing_jobs.go`,
`internal/jobs/notification_jobs.go`, `internal/domain/bill.go`, the `bills`/`bill_actions`
tables and `check_overdue_bills()`/`auto_resolve_disputed_bills()` functions in
`podman/trusted-group/postgres/ubertool_schema_trusted.sql`, and
`api/proto/ubertool_trusted_backend/v1/bill_split_service.proto` — cross-checked against
`docs/design/grpc_api_business_logic.md` and `docs/improvements/*`. It is **not** a
proposal for new behavior; every place source and docs disagreed is called out in "Known
Discrepancies" below rather than silently resolved.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Monthly Bill Generation & Notice Pipeline (Priority: P1)

Each month, the system nets every member's balance within an organization against every
other member's, generates the minimum set of payment obligations ("bills") needed to settle
them, and notifies the affected debtor and creditor by email.

**Why this priority**: Every other bill-split behavior (acknowledgment, disputes,
resolution) operates on bills this pipeline creates. Without it, nothing downstream has
data to act on.

**Independent Test**: Seed `users_orgs` balances for an org, run the settlement pipeline
(snapshot → netting → bill creation → notice email), and verify the resulting `bills` rows
match the expected debtor/creditor/amount set and that `notice_sent_at` gets stamped once
the notice email succeeds.

**Acceptance Scenarios**:

1. **Given** an org with members holding non-zero balances, **When** the monthly settlement
   job runs, **Then** `TakeBalanceSnapshots` records each member's balance for the
   settlement month, `PerformBillSplittingForOrg` computes net debtor→creditor transactions
   via a greedy largest-balance-first match (`CalculateTransactions`), and one `bills` row
   per transaction is inserted with `status = 'PENDING'` and `notice_sent_at = NULL`.
2. **Given** both the largest creditor and largest debtor balances in an org are below the
   org's `billsplit_settlement_threshold_cents`, **When** the netting algorithm runs,
   **Then** matching stops for that pair and no bill is created for amounts below
   threshold on both sides.
3. **Given** a `(org_id, debtor_user_id, creditor_user_id, settlement_month)` bill already
   exists (e.g. the job is re-run for a month that already settled), **When** the insert is
   attempted again, **Then** the duplicate is silently skipped (`ON CONFLICT DO NOTHING`) —
   no error, no duplicate bill.
4. **Given** a newly created `PENDING` bill with `notice_sent_at IS NULL`,
   **When** `SendBillSplittingNotices` runs, **Then** it emails both debtor and creditor and,
   **only if the debtor's email send succeeds**, stamps `notice_sent_at = NOW()` — a failed
   creditor email does not block the stamp, and a failed debtor email leaves the bill
   eligible to be retried on the next run.

---

### User Story 2 - Payment Acknowledgment Cycle (Priority: P1)

The debtor acknowledges having sent payment; the creditor then acknowledges having
received it. Once both acknowledge, the bill is settled and balances update. This also
works if the bill has already entered `DISPUTED` status — acknowledging in that state
provides a graceful path back to settlement without admin intervention.

**Why this priority**: This is the primary, most-used interaction in the feature — the
normal happy path every bill goes through.

**Independent Test**: Create a `PENDING` bill between two users; call `AcknowledgePayment`
as the debtor, then as the creditor; verify status becomes `PAID`, `resolution_outcome =
GRACEFUL`, and both users' `users_orgs.balance_cents` reflect the transfer.

**Acceptance Scenarios**:

1. **Given** a bill in `PENDING` or `DISPUTED` status with `debtor_acknowledged_at IS NULL`,
   **When** the debtor calls `AcknowledgePayment`, **Then** `debtor_acknowledged_at` is set,
   a `DEBTOR_ACKNOWLEDGED` `bill_actions` row is created, and the creditor is notified
   (in-app + email); bill status does **not** change yet.
2. **Given** a bill with `debtor_acknowledged_at` set and `creditor_acknowledged_at IS NULL`,
   in status `PENDING` or `DISPUTED`, **When** the creditor calls `AcknowledgePayment`,
   **Then** `creditor_acknowledged_at` is set, status becomes `PAID`, `resolved_at = NOW()`,
   `resolution_outcome = GRACEFUL`, a `CREDITOR_ACKNOWLEDGED` action is recorded, the
   creditor's balance increases by `amount_cents`, the debtor's balance decreases by
   `amount_cents`, and the debtor is notified.
3. **Given** a bill currently in `DISPUTED` status (see User Story 3) where neither party
   has acknowledged yet, **When** both debtor and creditor call `AcknowledgePayment` in
   sequence (debtor then creditor), **Then** the same graceful settlement in Scenario 2
   occurs — the bill resolves as `PAID`/`GRACEFUL` **without** any admin action, exactly as
   if it had never left `PENDING`.
4. **Given** a bill in any status other than `PENDING` or `DISPUTED` (`PAID`,
   `ADMIN_RESOLVED`, `SYSTEM_DEFAULT_ACTION`), **When** either party calls
   `AcknowledgePayment`, **Then** the call is rejected with an error and no state changes.
5. **Given** a party who already acknowledged, **When** they call `AcknowledgePayment`
   again, **Then** the call is rejected ("already acknowledged").
6. **Given** the debtor has not yet acknowledged, **When** the creditor calls
   `AcknowledgePayment`, **Then** the call is rejected ("debtor has not acknowledged
   payment yet").

---

### User Story 3 - Automated Dispute Lifecycle (Priority: P2)

If a bill sits unacknowledged too long, the system escalates it automatically: a reminder
email at 3 days after notice, an automatic move to `DISPUTED` at 10 days after notice, and
— if still unresolved by the end of the settlement month — an automatic forced resolution
that blocks both parties.

**Why this priority**: This is the enforcement mechanism that makes the whole bill-split
system self-driving; without it, disputes would require manual admin polling to ever
surface. It runs entirely in the background (no direct user-facing RPC), which is why it
ranks below the two directly user-triggered flows above.

**Independent Test**: Insert a bill with `notice_sent_at` set to 11 days ago and status
`PENDING`; run `CheckOverdueBills`; verify status becomes `DISPUTED` with
`dispute_reason` set appropriately and a `DISPUTE_OPENED` action recorded. Separately,
insert a `DISPUTED` bill for the current settlement month; run `ResolveDisputedBills`;
verify status becomes `SYSTEM_DEFAULT_ACTION`, `resolution_outcome = BOTH_FAULT`, and both
parties are blocked.

**Acceptance Scenarios**:

1. **Given** a `PENDING` bill with `notice_sent_at` more than 72 hours in the past,
   **When** `SendBillReminders` runs, **Then** both debtor and creditor receive a reminder
   email (bill status and `notice_sent_at` are unchanged).
2. **Given** a `PENDING` bill with `notice_sent_at` more than 10 days in the past and
   `disputed_at IS NULL`, **When** `CheckOverdueBills` runs (daily), **Then** the bill's
   status becomes `DISPUTED`, `disputed_at = NOW()`, `dispute_reason` is set to
   `DEBTOR_NO_ACK` if the debtor never acknowledged or `CREDITOR_NO_ACK` if only the
   creditor is missing, and a `DISPUTE_OPENED` `bill_actions` row (`actor_user_id = NULL`,
   i.e. system-originated) is created.
3. **Given** a bill still in `DISPUTED` status for a given `(org_id, settlement_month)` when
   `ResolveDisputedBills` runs for that org/month (scheduled for month-end), **Then** the
   bill's status becomes `SYSTEM_DEFAULT_ACTION`, `resolution_outcome = 'BOTH_FAULT'`,
   `resolution_notes = 'Auto-resolved by system at end of month - both parties blocked'`,
   `resolved_at = NOW()`, the debtor is blocked from renting
   (`users_orgs.renting_blocked = true`), the creditor is blocked from lending
   (`users_orgs.lending_blocked = true`), both with `blocked_due_to_bill_id` set to the
   bill, and a `SYSTEM_AUTO_RESOLVE` action is recorded. **Note**: unlike admin-driven
   `BOTH_FAULT` resolution (User Story 4), this system path does **not** apply a balance
   penalty to either party — only the blocking side-effects.
4. **Given** a bill whose `notice_sent_at` never got stamped (debtor notice email kept
   failing — see US1 Scenario 4), **When** `CheckOverdueBills` runs, **Then** the bill is
   *not* considered overdue (the 10-day clock only starts once `notice_sent_at` is set) —
   it remains `PENDING` indefinitely until a notice successfully sends.

---

### User Story 4 - Admin Dispute Resolution (Priority: P2)

An org admin (or super-admin) reviews disputes not involved with them and resolves each one
either by declaring fault (with balance and blocking consequences) or by confirming the
parties already settled offline (no penalty).

**Why this priority**: This is the manual-intervention safety valve for disputes the
automated lifecycle (US3) hasn't yet force-resolved, or that need a human judgment call
before month-end.

**Independent Test**: Create a `DISPUTED` bill; call `ListDisputedPayments` as an org admin
and confirm it appears; call `ResolveDispute` with each of the four resolution types and
verify the corresponding balance/blocking side effects.

**Acceptance Scenarios**:

1. **Given** the caller has `ADMIN` or `SUPER_ADMIN` role in the bill's org and is neither
   the debtor nor creditor on the bill, **and** the bill is in `DISPUTED` status,
   **When** `ResolveDispute` is called with `resolution = DEBTOR_FAULT`, **Then** balances
   are updated as a normal payment (creditor +amount, debtor −amount), the debtor is
   additionally blocked from renting with `blocked_due_to_bill_id` set, status becomes
   `ADMIN_RESOLVED`, and an `ADMIN_RESOLUTION` action is recorded with the admin's notes.
2. **Given** the same preconditions, **When** called with `resolution = CREDITOR_FAULT`,
   **Then** the creditor's balance is penalized by `amount_cents` (debtor's balance is
   **not** separately credited), the creditor is blocked from lending, and status becomes
   `ADMIN_RESOLVED`.
3. **Given** the same preconditions, **When** called with `resolution = BOTH_FAULT`,
   **Then** both debtor and creditor balances are penalized by `amount_cents` each, debtor
   is blocked from renting, creditor is blocked from lending, status becomes
   `ADMIN_RESOLVED`.
4. **Given** the same preconditions, **When** called with `resolution = GRACEFUL`,
   **Then** balances settle as a normal payment (no penalty, no blocking) — used when the
   admin confirms the parties already resolved it outside the app.
5. **Given** the caller is the debtor or creditor on the bill, **When** `ResolveDispute` is
   called, **Then** it is rejected ("admins cannot resolve disputes they are involved in").
6. **Given** the bill is not currently in `DISPUTED` status, **When** `ResolveDispute` is
   called, **Then** it is rejected ("payment is not in disputed status").
7. Both parties receive an in-app notification, a push notification, and an email
   describing the resolution outcome and the admin's notes, regardless of resolution type.

---

### User Story 5 - Bill Split Dashboards & History (Priority: P3)

Users and admins can see counts and lists of their bill-split activity: what they owe, what
they're owed, what's in dispute, and completed history.

**Why this priority**: Read-only visibility into the state the other stories produce;
valuable but not load-bearing for the settlement/dispute mechanics themselves.

**Independent Test**: Seed bills in various statuses for a user across two orgs; call the
summary and list RPCs and verify counts/contents match the seeded data.

**Acceptance Scenarios**:

1. **Given** a user with bills across multiple orgs, **When**
   `GetGlobalBillSplitSummary` is called, **Then** it returns, summed across all the user's
   orgs: count of `PENDING` bills where the user is debtor and hasn't acknowledged
   ("payments to make"), count of `PENDING` bills where the user is creditor and the debtor
   *has* acknowledged ("receipts to verify"), count of `DISPUTED` bills where the user is
   debtor ("payments in dispute"), and count of `DISPUTED` bills where the user is creditor
   ("receipts in dispute"). `GetOrganizationBillSplitSummary` returns the same breakdown
   per-org instead of summed.
2. **Given** `show_history = false`, **When** `ListPayments` is called, **Then** it returns
   only the caller's bills with status `PENDING` or `DISPUTED` in the given org.
3. **Given** `show_history = true`, **When** `ListPayments` is called, **Then** it returns
   only the caller's bills with status `PAID`, `ADMIN_RESOLVED`, or
   `SYSTEM_DEFAULT_ACTION` in the given org.
4. **Given** a `payment_id`, **When** `GetPaymentDetail` is called by the debtor, the
   creditor, or an org admin, **Then** it returns the bill, its full `bill_actions` history,
   and a computed `can_acknowledge` flag (true only when the caller is the party who still
   needs to acknowledge, per User Story 2's state rules). Any other caller is rejected as
   unauthorized.
5. **Given** the caller is an org admin, **When** `ListDisputedPayments` is called,
   **Then** it returns `DISPUTED` bills in the org **excluding** any bill where the calling
   admin is debtor or creditor.

---

### Edge Cases

- A bill notice email that keeps failing to send to the debtor leaves that bill's 10-day
  overdue clock never started (see US3 Scenario 4) — current behavior, not treated as a
  defect by this spec.
- Re-running the monthly settlement job for a month that already produced a given
  `(org, debtor, creditor)` bill silently no-ops for that pair (unique constraint +
  `ON CONFLICT DO NOTHING`); it does not update or duplicate the existing bill.
- `ListDisputedPayments`/`ListResolvedDisputes`/`ListPayments` as-built return the entire
  matching result set with no pagination or filtering applied, regardless of what the
  proto request fields suggest is possible (see Known Discrepancies).
- An admin who is a member of the org but not `ADMIN`/`SUPER_ADMIN` gets rejected from all
  admin-only RPCs (`ListDisputedPayments`, `ListResolvedDisputes`, `ResolveDispute`).
- A caller with no relationship to a bill (not debtor, creditor, or org admin) is rejected
  from `GetPaymentDetail`.

## Known Discrepancies *(code vs. documentation, verified against source)*

1. **Pagination/filtering is documented but not implemented.** Proto
   (`bill_split_service.proto`) defines `PaginationRequest`/`PaginationResponse` on
   `ListPayments`, `ListDisputedPayments`, and `ListResolvedDisputes`, plus a
   `settlement_month` filter on `ListPayments` and a `resolution_outcome` filter on
   `ListResolvedDisputes`. `docs/design/grpc_api_business_logic.md` documents this
   pagination/filtering behavior as if it works. **As-built**: neither
   `internal/api/grpc/bill_split.go` nor `internal/service/bill_split.go` reads
   `req.Pagination`, `req.SettlementMonth`, or `req.ResolutionOutcome` — these three RPCs
   always return the complete unfiltered, unpaginated result set for the caller.
2. **`docs/improvements/bill-split-api-review.md`'s "critical bug" claim is stale.** It
   states `AcknowledgePayment` only works when `status = PENDING`. As of current code,
   `acknowledgeAsDebtor`/`acknowledgeAsCreditor` both explicitly accept `PENDING` **or**
   `DISPUTED` (see User Story 2, Scenario 3) — the graceful-resolution-after-dispute path
   this document said was missing already exists. Treat that document as superseded.
3. **`ADMIN_COMMENT` is modeled but has no API.** `domain.BillActionTypeAdminComment` and
   `bill_actions.action_details` (JSONB) could support admins adding comments to a dispute,
   and `docs/improvements/bill-split-api-review.md` proposes an `AddDisputeComment` RPC for
   this — but no such service method or RPC exists in the current code. This spec describes
   as-built behavior only; adding this capability is explicitly out of scope here.
4. **The full notice/escalation/auto-resolution pipeline is undocumented.**
   `docs/design/grpc_api_business_logic.md`'s "Bill Split" section covers only the
   directly-invoked RPCs (summary, list, detail, acknowledge, admin dispute
   list/resolve). It does not mention `SendBillSplittingNotices`, `SendBillReminders`,
   `CheckOverdueBills`, or `ResolveDisputedBills` at all, even though these background jobs
   (registered in `internal/scheduler/scheduler.go`) drive every non-manual state
   transition a bill goes through (User Stories 1 and 3 of this spec). This spec closes
   that documentation gap; it is not proposing new functionality.

## Current Test Coverage Baseline *(informational — grounds the next /speckit-tasks pass, not a requirement)*

Verified by inspection of `tests/unit/bill_split_service_test.go`,
`tests/integration/bill_split_test.go`, `tests/e2e/bill_split_test.go`,
`tests/unit/billing_jobs_test.go`, `tests/integration/billing_jobs_integration_test.go`:

**Covered** (unit and/or integration and/or e2e): `GetGlobalBillSplitSummary`,
`GetOrganizationBillSplitSummary`, `ListPayments`, `GetPaymentDetail`,
`ListDisputedPayments`, `ListResolvedDisputes`, `ResolveDispute` (all three forced-fault
outcomes), unauthorized-access rejection, the `CalculateTransactions` netting algorithm, and
`PerformBillSplittingForOrg`.

**Not covered anywhere** (no unit, integration, or e2e test exercises these):

- `CheckOverdueBills` / the `check_overdue_bills()` SQL function (10-day auto-dispute
  transition, User Story 3 Scenario 2).
- `ResolveDisputedBills` / the `auto_resolve_disputed_bills()` SQL function (month-end
  auto-`SYSTEM_DEFAULT_ACTION` transition, User Story 3 Scenario 3).
- The graceful-resolution-after-dispute path of `AcknowledgePayment` (User Story 2,
  Scenario 3) — both parties acknowledging while status is already `DISPUTED`.
- `SendBillSplittingNotices` and `SendBillReminders` (User Story 1 Scenario 4, User Story 3
  Scenario 1), including the asymmetric debtor/creditor email-failure behavior around
  `notice_sent_at`.
- `AcknowledgePayment`'s rejection paths (already-acknowledged, wrong status, creditor
  acknowledging before debtor).

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The monthly settlement job MUST snapshot every member's balance, then compute
  net debtor→creditor transactions per org via the largest-balance-first greedy match, then
  create one `PENDING` bill per resulting transaction, skipping any pairing already settled
  for that org/month.
- **FR-002**: The system MUST email both debtor and creditor when a bill is created, and
  MUST record `notice_sent_at` only once the debtor's notice email send succeeds.
- **FR-003**: The system MUST send a reminder email to both parties of a `PENDING` bill once
  `notice_sent_at` is more than 72 hours in the past.
- **FR-004**: The system MUST automatically transition a `PENDING` bill to `DISPUTED`,
  recording a dispute reason and a system `bill_actions` entry, once `notice_sent_at` is
  more than 10 days in the past and no dispute has been opened yet.
- **FR-005**: The system MUST automatically transition any bill still `DISPUTED` at the
  scheduled month-end run for its org/settlement-month to `SYSTEM_DEFAULT_ACTION` with
  `resolution_outcome = BOTH_FAULT`, blocking the debtor from renting and the creditor from
  lending, without applying a balance penalty.
- **FR-006**: `AcknowledgePayment` MUST accept calls from the bill's debtor or creditor
  while status is `PENDING` or `DISPUTED`, MUST reject calls in any other status, MUST
  reject a party acknowledging twice, and MUST reject the creditor acknowledging before the
  debtor has.
- **FR-007**: When both debtor and creditor have acknowledged, the system MUST set status
  to `PAID`, `resolution_outcome = GRACEFUL`, transfer `amount_cents` between the two
  parties' `users_orgs.balance_cents`, and notify the debtor — regardless of whether the
  bill reached this point directly from `PENDING` or via `DISPUTED`.
- **FR-008**: `ResolveDispute` MUST be restricted to callers with `ADMIN`/`SUPER_ADMIN` role
  in the bill's org who are not themselves the debtor or creditor, and MUST only operate on
  bills currently in `DISPUTED` status.
- **FR-009**: `ResolveDispute` MUST apply the balance and blocking consequences documented
  in User Story 4 for each of `DEBTOR_FAULT`, `CREDITOR_FAULT`, `BOTH_FAULT`, and
  `GRACEFUL`, and MUST notify both parties (in-app, push, and email) of the outcome.
- **FR-010**: `ListPayments` MUST filter to the caller's own bills in the given org, using
  `show_history` to choose between active (`PENDING`/`DISPUTED`) and completed
  (`PAID`/`ADMIN_RESOLVED`/`SYSTEM_DEFAULT_ACTION`) statuses.
- **FR-011**: `GetPaymentDetail` MUST be restricted to the bill's debtor, creditor, or an
  org admin, and MUST compute `can_acknowledge` per the rules in User Story 2.
- **FR-012**: `ListDisputedPayments` and `ListResolvedDisputes` MUST be restricted to org
  admins; `ListDisputedPayments` MUST exclude bills the calling admin is a party to.
- **FR-013**: As-built, `ListPayments`, `ListDisputedPayments`, and `ListResolvedDisputes`
  MUST return their full matching result set — pagination and the `settlement_month`/
  `resolution_outcome` filters are accepted on the wire but not applied (see Known
  Discrepancy 1). Any future work that actually implements filtering/pagination for these
  RPCs supersedes this requirement.
- **FR-014** *(multi-org requirement from `grpc_api_business_logic.md` Bill Split section)*:
  `GetGlobalBillSplitSummary` MUST return aggregated bill-split counts summed across ALL
  organizations the caller belongs to. The response MUST include: `payments_to_make` (count
  of `PENDING` bills where caller is debtor and hasn't acknowledged), `receipts_to_verify`
  (count of `PENDING` bills where caller is creditor and debtor has acknowledged),
  `payments_in_dispute` (count of `DISPUTED` bills where caller is debtor), and
  `receipts_in_dispute` (count of `DISPUTED` bills where caller is creditor). This provides
  a cross-org dashboard view without requiring per-org calls.
- **FR-015** *(multi-org requirement from `grpc_api_business_logic.md` Bill Split section)*:
  `GetOrganizationBillSplitSummary` MUST return the same four count categories as
  `GetGlobalBillSplitSummary`, but broken down PER ORGANIZATION the caller belongs to.
  Each entry in the list MUST include the `organization_id`, `organization_name`, and the
  four counts for that specific org. This enables per-org drill-down from the global summary.
- **FR-016** *(feature extension)*:
  `GetGlobalBillSplitSummary` MUST return `bills_created_count` — the total number of bills
  across **all statuses** (not just PENDING/DISPUTED) where the caller is either debtor or
  creditor, summed across all organizations the caller belongs to. This provides a single
  number representing the caller's total bill activity, distinct from the four category
  counts which only cover active (PENDING/DISPUTED) bills.

### Key Entities

- **Bill**: A single debtor→creditor payment obligation for a settlement month. Fields of
  record: amount, org, debtor/creditor, status (`PENDING`/`PAID`/`DISPUTED`/
  `ADMIN_RESOLVED`/`SYSTEM_DEFAULT_ACTION`), notice/acknowledgment/dispute/resolution
  timestamps, dispute reason, resolution outcome and notes. Unique per
  `(org, debtor, creditor, settlement_month)`.
- **BillAction**: An immutable audit-log entry against a bill (`NOTICE_SENT`,
  `DEBTOR_ACKNOWLEDGED`, `CREDITOR_ACKNOWLEDGED`, `DISPUTE_OPENED`, `ADMIN_COMMENT`
  (modeled, unused — see Known Discrepancy 3), `ADMIN_RESOLUTION`,
  `SYSTEM_AUTO_RESOLVE`), with a nullable actor (null = system-originated).
- **Organization Settlement Threshold**: `orgs.billsplit_settlement_threshold_cents` —
  the minimum balance the netting algorithm requires on *both* sides of a pairing to keep
  matching (see User Story 1 Scenario 2).
- **UsersOrgs blocking fields**: `renting_blocked`, `lending_blocked`,
  `blocked_due_to_bill_id`, `blocked_reason` — the consequence state a bill's resolution can
  write to a member's org membership.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001 — `DISPUTED→ADMIN_RESOLVED` ×4 outcomes MET at the unit-test level 2026-07-22**:
  Every bill status transition documented in this spec (`PENDING→DISPUTED`, `PENDING→PAID`,
  `DISPUTED→PAID`, `DISPUTED→ADMIN_RESOLVED` ×4 outcomes, `DISPUTED→SYSTEM_DEFAULT_ACTION`) is
  exercised by at least one automated test that asserts the resulting status, timestamps, and
  balance/blocking side effects. Prior to this date, the `GRACEFUL` outcome of
  `DISPUTED→ADMIN_RESOLVED` had zero test evidence at any tier — only `DEBTOR_FAULT`,
  `CREDITOR_FAULT`, and `BOTH_FAULT` were covered — making this claim false as literally
  written; see `TestBillSplitService_ResolveDispute/Success_Graceful` and
  `sbr/rtm/008-bill-split.rtm.md`. `DISPUTED→SYSTEM_DEFAULT_ACTION` (the automated
  `ResolveDisputedBills` job) remains uncovered — tracked separately as Phase 2 (FR-005) in
  `sbr/remediation-plan.md`, not claimed as met here.
- **SC-002**: The two automated lifecycle jobs (`CheckOverdueBills`,
  `ResolveDisputedBills`) each have an automated test that seeds a bill in the qualifying
  state, runs the job, and verifies the database-level outcome — not just that the job
  runs without error.
- **SC-003**: The graceful-resolution-after-dispute path (User Story 2 Scenario 3) has a
  dedicated automated test distinct from the plain `PENDING`-only acknowledgment tests
  already in place.
- **SC-004**: All four items in "Known Discrepancies" are either corrected in
  `docs/design/grpc_api_business_logic.md` (or the relevant `docs/improvements/*` file) as
  a follow-up task, or explicitly marked "as-built, no doc change needed" — none remain
  silently unresolved after this spec's follow-up tasks complete.
- **SC-005**: A developer reading only this spec (not the source) can correctly predict,
  for any of the seven bill-split RPCs, whether its response is paginated/filtered as
  requested.

## Assumptions

- Email delivery for bill notices/reminders is assumed to eventually succeed; a bill stuck
  because its debtor notice email keeps failing is treated as acceptable current behavior
  (US3 Scenario 4), not a defect requiring a fix in this spec's scope.
- The cron schedules for the six background jobs (`SendBillReminders`, `CheckOverdueBills`,
  `ResolveDisputedBills`, `TakeBalanceSnapshots`, `PerformBillSplitting`,
  `SendBillSplittingNotices`) are environment configuration (`internal/config`) and are out
  of scope for this spec, which describes job *behavior* once triggered, not *cadence*.
  Both deployment targets (Podman local, EC2) run the same job set per Constitution
  Principle V.
- This spec covers only the bill-split-specific notice/reminder jobs in
  `internal/jobs/notification_jobs.go` (`SendBillReminders`, `SendBillSplittingNotices`);
  the general Notifications domain (in-app notifications, push, `fcm_tokens`) is covered
  separately in a future Notifications-domain spec.
- "As-built" pagination/filtering behavior (Known Discrepancy 1, FR-013) is documented as
  current fact, not endorsed as correct; whether to implement it or to change the proto/doc
  instead is a product decision left to a follow-up task, not decided by this spec.
