# Feature Specification: Ledger (As-Built)

**Feature Branch**: `008-ledger`

**Created**: 2026-07-22

**Status**: Draft

**Input**: Retrofit specification for the existing, already-implemented and deployed
Ledger feature — the smallest domain in this codebase by RPC count (3), but one that
turned up more confirmed doc-vs-code drift per line than any other domain reviewed so far.
Per project constitution Principle I ("Code Is Truth"), this document describes verified
current behavior of `internal/service/ledger.go`, `internal/api/grpc/ledger.go`,
`internal/repository/postgres/ledger.go`, the `ledger_transactions` table in
`podman/trusted-group/postgres/ubertool_schema_trusted.sql`, and
`api/proto/ubertool_trusted_backend/v1/ledger_service.proto` — cross-checked against
`docs/design/grpc_api_business_logic.md`'s "Ledger" section. It is **not** a proposal for
new behavior; every gap found is called out in "Known Discrepancies" below.

Ledger is **read-only** from this domain's own RPC surface — the only mutation,
`CreateTransaction`, is called exclusively by other domains (Rentals' `CompleteRental`, see
`specs/006-rentals/spec.md`) and is not itself exposed as an RPC here.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Check Current Balance (Priority: P1)

A user checks their credit/debit balance within a specific organization.

**Why this priority**: The single most consulted piece of financial state in the app —
every other RPC in this domain elaborates on it.

**Independent Test**: Seed a `users_orgs.balance_cents` value for a user in an org; call
`GetBalance`; confirm the returned amount matches.

**Acceptance Scenarios**:

1. **Given** an authenticated user and an `organization_id` they belong to, **When**
   `GetBalance` is called, **Then** `users_orgs.balance_cents` for that
   `(user_id, org_id)` pair is returned.
2. **Given** the response is mapped back to the client, **When** `GetBalanceResponse` is
   constructed, **Then** — **as-built** — `last_updated_on` is always an empty string,
   regardless of the actual `users_orgs.last_balance_updated_on` value (see Known
   Discrepancy 1).

---

### User Story 2 - Browse Transaction History (Priority: P2)

A user pages through their ledger transaction history within an organization.

**Why this priority**: Detail view behind the balance/summary; used less often than
checking the current balance itself.

**Independent Test**: Seed several `ledger_transactions` rows for a user/org; call
`GetTransactions` with pagination; confirm ordering (most recent first) and `total_count`.

**Acceptance Scenarios**:

1. **Given** an authenticated user and `organization_id`, **When** `GetTransactions` is
   called, **Then** that user's transactions in that org are returned ordered by
   `created_on` descending, paginated by `page`/`page_size`, alongside a `total_count` of
   all matching rows (not just the current page).

---

### User Story 3 - Ledger Summary Dashboard (Priority: P2)

A user gets a consolidated view of their balance plus rental activity counts for an
organization.

**Why this priority**: A dashboard aggregation used for at-a-glance status, not a
transactional flow — but documented with the richest (and least accurately implemented)
business logic in this domain.

**Independent Test**: Seed a balance and several rentals in various statuses for a user in
an org; call `GetLedgerSummary`; confirm `balance` and `status_count` match. Separately, seed
a balance and rentals across two orgs for the same user and call it with `organization_id`
omitted; confirm `balance` and `status_count` reflect the sum across both orgs.

**Acceptance Scenarios**:

1. **Given** an authenticated user and an `organization_id`, **When**
   `GetLedgerSummary` is called, **Then** the response includes that org's balance
   (identical lookup to `GetBalance`) and a `status_count` map of the user's rentals in
   that org (as both renter and owner) grouped by `RentalStatus`.
2. **Given** `organization_id` is omitted (zero value), **When** `GetLedgerSummary` is
   called, **Then** the balance returned is the sum of `balance_cents` across every
   `users_orgs` row for the caller, and the per-status rental counts are aggregated across
   every org the caller belongs to (as renter or owner).
3. **Given** a `number_of_months` value greater than zero is supplied, **When**
   `GetLedgerSummary` is called, **Then** the rental-activity counts (`ActiveRentalsCount`,
   `ActiveLendingsCount`, `PendingRequestsCount`, and the `status_count` map) are limited to
   rentals with `created_on` within that many months of today — for both the single-org
   path (`GetSummary`) and the cross-org rollup (`GetSummaryAllOrgs`), applied per-org
   before aggregation. `balance` is never affected by this filter. **Given**
   `number_of_months` is omitted (zero value), **When** `GetLedgerSummary` is called,
   **Then** rental-activity counts reflect the caller's entire history, unfiltered — this
   preserves the pre-existing behavior for every caller that does not set the field
   (Known Discrepancy 2 resolved).
4. **Given** the response is constructed, **When** the client expects "recent
   transactions" as part of the summary (per documentation), **Then** — **as-built** —
   none are included; `GetLedgerSummaryResponse` carries only `balance` and
   `status_count` (see Known Discrepancy 3).

---

### Edge Cases

- `GetLedgerSummary`'s rental-activity counts (`ActiveRentalsCount`, `ActiveLendingsCount`,
  `PendingRequestsCount`, and the full `status_count` map) all query the `rentals` table
  directly by `org_id`, independently of anything in `specs/006-rentals/spec.md`'s
  documented status machine — if a new `RentalStatus` value were ever introduced there,
  it would automatically appear in `status_count` here with no code change needed (the
  `GROUP BY status` query is generic), but the three named counts
  (`ActiveRentalsCount`/`ActiveLendingsCount`/`PendingRequestsCount`) are hardcoded to the
  literal strings `'ACTIVE'`/`'PENDING'` and would not track a renamed status value.
- `GetBalance` (the standalone RPC) has no rollup for `organization_id = 0` and is not
  documented to have one — only `GetLedgerSummary` rolls up, via a separate code path
  (`ledgerRepository.GetSummaryAllOrgs`) that does not call `GetBalance`.

## Known Discrepancies *(code vs. documentation, verified against source)*

1. **`GetBalance` never populates the documented `last_updated_on` field.**
   `grpc_api_business_logic.md`'s "Get Balance" step 2 says: "return
   `users_orgs.balance_cents` **and** `user_orgs.last_balance_updated_on`." The proto
   (`GetBalanceResponse`) has a `last_updated_on` field for exactly this. **As-built**,
   `ledgerRepository.GetBalance` (`internal/repository/postgres/ledger.go`) only selects
   `balance_cents`; `LedgerHandler.GetBalance` (`internal/api/grpc/ledger.go`) constructs
   `&pb.GetBalanceResponse{Balance: balance}` with `LastUpdatedOn` never set — the field is
   always an empty string in every response, regardless of the actual database value.
2. ~~**`number_of_months` is accepted by the proto but read by nothing, for either the
   single-org or cross-org path.**~~ **Resolved.** `LedgerHandler.GetLedgerSummary` now
   forwards `req.NumberOfMonths` through `LedgerService.GetLedgerSummary` to both
   `ledgerRepository.GetSummary` (single org) and `ledgerRepository.GetSummaryAllOrgs`
   (cross-org rollup), each of which adds a `created_on >= (CURRENT_DATE - (N *
   INTERVAL '1 month'))` condition to all four rental-activity queries
   (`ActiveRentalsCount`, `ActiveLendingsCount`, `PendingRequestsCount`, `status_count`).
   `number_of_months <= 0` (including the proto3 zero-value default when the field is
   omitted) applies no filter, preserving full-history behavior for existing callers.
   `balance` is unaffected in both paths.
3. **The documented "recent transactions" output is not part of the actual response.**
   `grpc_api_business_logic.md` lists "balance, recent transactions, and activity counts"
   as the output of `GetLedgerSummary`. The proto's `GetLedgerSummaryResponse` message
   defines only `balance` and `status_count` — there is no transactions field to populate,
   and no code path attempts to include any.

## Current Test Coverage Baseline *(informational — grounds the next /speckit-tasks pass, not a requirement)*

Verified by inspection of `tests/unit/ledger_service_test.go`,
`tests/integration/ledger_test.go`, and `tests/e2e/ledger_test.go`:

**Covered**: `GetBalance` and `GetTransactions` happy paths (unit); `GetBalance` and
`GetLedgerSummary` end-to-end with a concrete `organization_id`, including a
before/after-settlement balance comparison exercised through the Rentals completion flow
(e2e); `GetLedgerSummary` called with `organization_id` omitted, rolling up balance and
per-status rental counts across multiple orgs, at all three tiers
(`TestLedgerService_GetLedgerSummary_RollsUpAcrossOrgs`,
`TestLedgerRepository_GetSummary_CrossOrgRollup`, `TestLedgerService_E2E` > "GetLedgerSummary
rolls up across all orgs when organization_id is omitted"); `number_of_months` date-window
filtering, both single-org and cross-org, at all three tiers
(`TestLedgerService_GetLedgerSummary_FiltersByMonths`,
`TestLedgerRepository_GetSummary_FiltersByMonths`,
`TestLedgerRepository_GetSummaryAllOrgs_FiltersByMonths`, `TestLedgerService_E2E` >
"GetLedgerSummary applies number_of_months as a real filter") — each proving both the
`number_of_months=0` unbounded-history case and a positive window excluding old data.

**Not covered anywhere**:

- `GetBalance`'s `last_updated_on` field being empty (Known Discrepancy 1) — no test
  asserts on this field at all.
- `GetTransactions`'s pagination boundaries (exact `total_count` across multiple pages,
  empty-result page).

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: `GetBalance` MUST return the caller's `balance_cents` for the given org.
  **As-built, it MUST NOT be assumed to also return a populated `last_updated_on`** (Known
  Discrepancy 1) — this is the target correctness bar for a follow-up task, not current
  behavior.
- **FR-002**: `GetTransactions` MUST return only the caller's own transactions in the given
  org, most recent first, with an accurate `total_count` independent of the current page.
- **FR-003**: `GetLedgerSummary` MUST return the caller's balance and a per-status count of
  their rentals (as renter or owner) in the given org, limited to rentals created within
  the last `number_of_months` months when that value is greater than zero (unbounded
  history when omitted). **As-built, it MUST NOT be assumed to include recent
  transactions** (Known Discrepancy 3) — that remains undocumented as dead scope.
- **FR-004** *(multi-org requirement from PRD 3.1, `grpc_api_business_logic.md` "Get Ledger
  Summary" step 1)*: `GetLedgerSummary` MUST roll up balances and rental counts across ALL
  organizations the caller belongs to when `organization_id` is omitted (zero value): (a)
  the sum of `balance_cents` across all the caller's `users_orgs` rows, and (b) per-status
  rental counts aggregated across all orgs. (c) The same `number_of_months` filtering is
  applied per-org before aggregation, matching FR-003's single-org behavior.

### Key Entities

- **LedgerTransaction**: `ledger_transactions` table — `org_id`, `user_id`, `amount`
  (signed cents), `type` (e.g. `LENDING_CREDIT`/`LENDING_DEBIT`, defined per-domain by
  whatever created the transaction), `related_rental_id`, `description`, `charged_on`.
  Created only by other domains (Rentals); this domain only reads it.
- **LedgerSummary**: an in-memory aggregation (not its own table) — `Balance`,
  `ActiveRentalsCount`, `ActiveLendingsCount`, `PendingRequestsCount`, `StatusCount` (full
  per-status breakdown), all computed live from `users_orgs` and `rentals` at request time.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: `GetBalance`'s `last_updated_on` field either gets populated from
  `users_orgs.last_balance_updated_on`, or the proto/doc is corrected to remove it — Known
  Discrepancy 1 does not remain a silently-always-empty field.
- **SC-002**: `GetLedgerSummary` implements the documented cross-org rollup (balance and
  rental counts) for omitted `organization_id`, including the `number_of_months` filter
  applied per-org before aggregation (SC-003).
- **SC-003**: ~~`number_of_months` either gains real filtering behavior, or is removed from
  the proto/doc as dead input~~ — **Met.** `number_of_months` now filters rental-activity
  counts by `created_on` for both the single-org and cross-org paths; Known Discrepancy 2
  no longer applies.
- **SC-004**: A developer reading only this spec can correctly predict every field in
  `GetBalanceResponse` and `GetLedgerSummaryResponse` without needing to read the source.

## Assumptions

- `CreateTransaction` (how and when ledger rows are created) is specified in the domain
  that calls it — currently only `specs/006-rentals/spec.md`'s `CompleteRental` flow; a
  future Bill Split ledger-writing path, if any is added, would extend that spec, not this
  one.
- Known Discrepancies 1 and 3 remain independent, low-risk changes (populate an existing
  column read; add a transactions field) not yet applied — consistent with their lower
  severity relative to the authorization/data-integrity findings fixed earlier in this
  retrofit (Organizations & Administration, Tools). Known Discrepancy 2 (`number_of_months`
  filtering) has been fixed as part of the FR-004(c) multi-org rollup work.
