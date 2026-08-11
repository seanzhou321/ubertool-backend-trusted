# Feature Specification: Ledger (As-Built)

**Feature Branch**: `008-ledger`

**Created**: 2026-07-22

**Status**: Draft

**Input**: Retrofit specification for the existing, already-implemented and deployed
Ledger feature — the smallest domain in this codebase by RPC count (3), but one that
turned up more confirmed doc-vs-code drift per line than any other domain reviewed so far.
Per project constitution Principle I ("Reconcile Discrepancies Among Spec, RTM, and Code"), this document describes verified
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
   constructed, **Then** `last_updated_on` reflects `users_orgs.last_balance_updated_on`
   (formatted `YYYY-MM-DD`), or an empty string if that column is `NULL` — e.g. a user who
   has never had a balance-changing transaction (Known Discrepancy 1 resolved).

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
   transactions" as part of the summary (per documentation), **Then**
   `GetLedgerSummaryResponse.recent_transactions` carries the caller's 5 most-recent
   `ledger_transactions` rows (scoped to the same org as the rest of the summary, or
   across all the caller's orgs when `organization_id` is omitted), most recent first
   (Known Discrepancy 3 resolved). This field is **not** filtered by `number_of_months` —
   it always reflects the most recent 5 transactions regardless of the summary's
   rental-activity window.

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

1. ~~**`GetBalance` never populates the documented `last_updated_on` field.**~~
   **Resolved.** `ledgerRepository.GetBalance` (`internal/repository/postgres/ledger.go`)
   now selects `COALESCE(last_balance_updated_on::text, '')` alongside `balance_cents` and
   returns it through `LedgerService.GetBalance`; `LedgerHandler.GetBalance`
   (`internal/api/grpc/ledger.go`) sets `LastUpdatedOn` on the response from that value.
   `last_updated_on` is an empty string only when the column is genuinely `NULL` (a user
   who has never had a balance-changing transaction), matching `grpc_api_business_logic.md`'s
   "Get Balance" step 2.
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
3. ~~**The documented "recent transactions" output is not part of the actual response.**~~
   **Resolved.** `GetLedgerSummaryResponse` now has a `recent_transactions` field (proto
   field 3). `ledgerRepository.GetSummary` and `ledgerRepository.GetSummaryAllOrgs` both
   call a shared `getRecentTransactions` helper that selects the 5 most-recent
   `ledger_transactions` rows for the caller (scoped to org for `GetSummary`, across all
   orgs for `GetSummaryAllOrgs`), most recent first; `MapDomainLedgerSummaryToProto`
   (`internal/api/grpc/mapper.go`) maps them onto the response. This field is a fixed
   "last 5" snapshot, independent of `number_of_months`.

## Current Test Coverage Baseline *(informational — grounds the next /speckit-tasks pass, not a requirement)*

Verified by inspection of `tests/unit/ledger_service_test.go`,
`tests/unit/repos/ledger_test.go`, `tests/integration/ledger_test.go`, and
`tests/e2e/ledger_test.go`:

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
`number_of_months=0` unbounded-history case and a positive window excluding old data;
`GetBalance.last_updated_on` reflecting `users_orgs.last_balance_updated_on` (or empty
string when `NULL`), at all three tiers
(`TestLedgerService_GetBalance_ReturnsLastUpdatedOn`,
`TestLedgerRepository_GetBalance_IncludesLastUpdatedOn`, `TestLedgerService_E2E` >
"GetBalance_LastUpdatedOn"); `GetLedgerSummary.recent_transactions` capped at 5,
most-recent-first, both single-org and cross-org, at all three tiers
(`TestLedgerService_GetLedgerSummary_IncludesRecentTransactions`,
`TestLedgerRepository_GetSummary_IncludesRecentTransactions`,
`TestLedgerRepository_GetSummaryAllOrgs_IncludesRecentTransactions`,
`TestLedgerService_E2E` > "GetLedgerSummary_RecentTransactions").

**Not covered anywhere**:

- `GetTransactions`'s pagination boundaries beyond the two-page case already covered
  (page sizes larger than the total count, an out-of-range page number).

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: `GetBalance` MUST return the caller's `balance_cents` for the given org,
  along with `last_updated_on` populated from `users_orgs.last_balance_updated_on`
  (formatted `YYYY-MM-DD`, empty string if `NULL`) (Known Discrepancy 1 resolved).
- **FR-002**: `GetTransactions` MUST return only the caller's own transactions in the given
  org, most recent first, with an accurate `total_count` independent of the current page.
- **FR-003**: `GetLedgerSummary` MUST return the caller's balance and a per-status count of
  their rentals (as renter or owner) in the given org, limited to rentals created within
  the last `number_of_months` months when that value is greater than zero (unbounded
  history when omitted), plus `recent_transactions` populated with the caller's 5
  most-recent `ledger_transactions` rows in that org, most recent first — unaffected by
  `number_of_months` (Known Discrepancy 3 resolved).
- **FR-004** *(multi-org requirement from PRD 3.1, `grpc_api_business_logic.md` "Get Ledger
  Summary" step 1)*: `GetLedgerSummary` MUST roll up balances and rental counts across ALL
  organizations the caller belongs to when `organization_id` is omitted (zero value): (a)
  the sum of `balance_cents` across all the caller's `users_orgs` rows, and (b) per-status
  rental counts aggregated across all orgs. (c) The same `number_of_months` filtering is
  applied per-org before aggregation, matching FR-003's single-org behavior. (d)
  `recent_transactions` is populated with the caller's 5 most-recent `ledger_transactions`
  rows across ALL their orgs (not scoped to a single org), most recent first.

### Key Entities

- **LedgerTransaction**: `ledger_transactions` table — `org_id`, `user_id`, `amount`
  (signed cents), `type` (e.g. `LENDING_CREDIT`/`LENDING_DEBIT`, defined per-domain by
  whatever created the transaction), `related_rental_id`, `description`, `charged_on`.
  Created only by other domains (Rentals); this domain only reads it.
- **LedgerSummary**: an in-memory aggregation (not its own table) — `Balance`,
  `ActiveRentalsCount`, `ActiveLendingsCount`, `PendingRequestsCount`, `StatusCount` (full
  per-status breakdown), `RecentTransactions` (up to 5 most-recent `LedgerTransaction`
  rows), all computed live from `users_orgs`, `rentals`, and `ledger_transactions` at
  request time.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: ~~`GetBalance`'s `last_updated_on` field either gets populated from
  `users_orgs.last_balance_updated_on`, or the proto/doc is corrected to remove it~~ —
  **Met.** `last_updated_on` now reflects the database column; Known Discrepancy 1 no
  longer applies.
- **SC-002**: `GetLedgerSummary` implements the documented cross-org rollup (balance and
  rental counts) for omitted `organization_id`, including the `number_of_months` filter
  applied per-org before aggregation (SC-003).
- **SC-003**: ~~`number_of_months` either gains real filtering behavior, or is removed from
  the proto/doc as dead input~~ — **Met.** `number_of_months` now filters rental-activity
  counts by `created_on` for both the single-org and cross-org paths; Known Discrepancy 2
  no longer applies.
- **SC-004**: A developer reading only this spec can correctly predict every field in
  `GetBalanceResponse` and `GetLedgerSummaryResponse` without needing to read the source —
  **Met**, now that `last_updated_on` and `recent_transactions` are both documented and
  implemented.
- **SC-005**: ~~`GetLedgerSummaryResponse` either gains a `recent_transactions` field
  backed by real data, or the proto/doc is corrected to remove the documented promise~~ —
  **Met.** `recent_transactions` now returns the caller's 5 most-recent transactions,
  most recent first; Known Discrepancy 3 no longer applies.

## Assumptions

- `CreateTransaction` (how and when ledger rows are created) is specified in the domain
  that calls it — currently only `specs/006-rentals/spec.md`'s `CompleteRental` flow; a
  future Bill Split ledger-writing path, if any is added, would extend that spec, not this
  one.
- Known Discrepancies 1, 2, and 3 have all been fixed: `last_updated_on` now reflects
  `users_orgs.last_balance_updated_on`; `number_of_months` filters rental-activity counts
  per-org before aggregation (FR-004(c)); `recent_transactions` returns the caller's 5
  most-recent transactions, most recent first, for both the single-org and cross-org
  paths. Adding `recent_transactions` required a proto change (`GetLedgerSummaryResponse`
  field 3) and regeneration via `make proto-gen` — a breaking change for any client relying
  on wire-compatibility with the prior message shape, though additive (a new field number)
  so existing clients that don't read it are unaffected.
- `recent_transactions` is intentionally a fixed "last 5" snapshot uncoupled from
  `number_of_months` — the design doc's "recent transactions" language never tied it to
  the same activity window as the rental-status counts, and a snapshot independent of the
  summary's date filter is the more useful dashboard behavior (a user filtering rental
  history to 3 months still wants to see their actual most recent transactions, not none
  if they happen to predate the window).
