# Requirements Traceability Matrix — Users

- **Source spec**: `specs/002-users/spec.md`
- **Tier mapping in effect**: `sbr/README.md` → "This repo's adapter" (L1=`tests/unit`,
  L2=`tests/integration`, L3=`tests/e2e`, Grounding=`tests/smoke`+`tests/e2e` journeys)
- **Generated**: 2026-07-22 by `speckit-sbr-audit`
- **Mode**: retrofit audit (as-built) — see project constitution Principle I

| FR-ID | Requirement Summary | L1 Unit | L2 Integration | L3 E2E | Grounding (Smoke) | Boundary Status | Notes |
|---|---|---|---|---|---|---|---|
| FR-001 | `GetUser` MUST return the caller's own profile (identity from JWT) joined with every org they belong to, including per-org role and balance. | — | — | `TestUserService_E2E > "GetUser Profile"` (`tests/e2e/user_test.go:23`) — asserts `id`/`email`/`name` and per-org `UserBalance` for 2 orgs | `TestSmoke... > "UsersTableReachable_via_Login"` (`tests/smoke/smoke_test.go:142`) — proves `users` table is reachable at all, not `GetUser`-specific behavior | **Gap — L1, L2 (L3 partial)** | No unit or integration test exercises `userService.GetUserProfile` (the function that assembles user+orgs+role, `internal/service/user.go:21`) in isolation. The e2e test verifies `UserBalance` per org but **never asserts the `org.Role` field** — half of the "per-org role and balance" MUST clause has zero assertion anywhere, despite spec.md's own Coverage Baseline listing "`GetUser` returning a profile with org memberships (e2e)" as fully "Covered." |
| FR-002 | `GetUser` MUST NOT fail outright when one membership's org record can't be loaded — MUST return the remaining memberships. | — | — | — | — | **Gap — all tiers** | No test anywhere constructs a scenario where `orgRepo.GetByID` fails for one of several memberships. The `continue`-on-error logic in `userService.GetUserProfile` (`internal/service/user.go:33-36`) that implements this MUST clause is entirely unexercised. Confirmed independently (grep for org-lookup-failure/partial-result scenarios in `tests/e2e/user_test.go` and `tests/unit` returned nothing) — matches spec.md's own Coverage Baseline, which lists this as "Not covered anywhere." |
| FR-003 | `UpdateProfile` MUST overwrite `name`, `email`, `phone`, `avatar_url` on the caller's row from the request, with no partial-update behavior. | — | `TestUserRepository_Integration > "Update"` (`tests/integration/user_test.go:61`) — repo-level only, changes a single field (`Name`), not a full 4-field overwrite | `TestUserService_E2E > "UpdateProfile"` (`tests/e2e/user_test.go:65`) — sets and verifies all 4 fields via non-empty values | — | **Gap — L1 (L2 partial, L3 partial)** | No unit test of `userService.UpdateProfile` itself. The integration test only proves a single-field DB update round-trips; it does not exercise the service-layer function that unconditionally overwrites all 4 fields. The e2e test proves the happy-path overwrite with non-empty values but — as spec.md's own Coverage Baseline states — **no test verifies the "no partial-update" semantics**, i.e. that omitting/blanking a field clears it rather than preserving the prior value. |
| FR-004 | `UpdateProfile` MUST reject a write that would violate the `users.email` `UNIQUE` constraint. | — | — | `TestUserService_E2E > "UpdateProfile Email Uniqueness"` (`tests/e2e/user_test.go:115`) — creates two users, attempts to update user2's email to user1's exact-case email, asserts `assert.Error(t, err)` | — | **Gap — L1, L2** | The rejection itself (the MUST clause) is verified at L3, but only by a generic `assert.Error` — no unit test isolates the service/repo behavior, and no integration test creates a duplicate-email conflict directly against Postgres. (The `docs/design` gap around case-sensitivity vs. `Signup`'s case-insensitive check — Known Discrepancy 2 — has zero test coverage at any tier, confirmed by grep; that gap is outside FR-004's own wording but adjacent.) |
| FR-005 | As-built, `UpdateProfile` MUST NOT be assumed to validate email format or reject empty `name`/`phone`/`email` beyond DB `NOT NULL` constraints. | — | — | — | — | **Gap — all tiers** | Zero test anywhere calls `UpdateProfile` with an empty/malformed `email`, empty `name`, or empty `phone` (confirmed by grep across `tests/e2e/user_test.go` and `tests/integration` — no matches). This is a "negative"/absence requirement: the only evidence that would satisfy it is a regression-lock test proving malformed input is currently accepted. None exists, so this documented gap in application behavior could be silently closed (someone adds validation) or silently widened without any test failing either way. Matches spec.md's own Coverage Baseline, which lists this as "Not covered anywhere." |

## Summary

- **5 FR-IDs audited, 0 fully `Complete`.**
- **Gap count by tier**: L1 — 5 (all FRs, no unit test of `userService.GetUserProfile` or
  `userService.UpdateProfile` exists at all); L2 — 5 (all FRs; `tests/integration` only
  exercises the repository layer directly — `Create`/`GetByEmail`/`Update` — never the
  service/RPC surface); L3 — 2 full gaps (FR-002, FR-005) plus 3 partial (FR-001's `role`
  field unasserted, FR-003's blank-clears-value semantics untested, FR-004's rejection
  proven only via generic `assert.Error`); Grounding — 5 (no FR has behavior-specific smoke
  evidence, which is expected — smoke targets deployment liveness, not per-requirement
  behavior).
- **FR-IDs with gaps**: FR-001, FR-002, FR-003, FR-004, FR-005 (all five).
- **Highest-priority finding**: FR-002 and FR-005 have **zero** test evidence at any tier —
  the silent-skip-on-org-lookup-failure logic (FR-002) and the documented absence of input
  validation (FR-005) are both entirely unverified, meaning either could regress (or, for
  FR-005, be silently "fixed" in a way that contradicts the spec) without any test noticing.
  Beyond what spec.md's own Coverage Baseline already self-reports, this audit surfaces one
  **new** discrepancy: spec.md's baseline claims "`GetUser` returning a profile with org
  memberships (e2e)" is "Covered," but the only e2e assertion of per-org data checks
  `UserBalance` — it never asserts the `role` field that FR-001 explicitly requires
  ("including per-org role and balance"). Unlike spec 003's finding, this is not a
  contradiction of an explicit Success-Criteria claim — spec.md is otherwise unusually
  candid about its own gaps (Known Discrepancies 1-4, an explicit Coverage Baseline section)
  — but it is a real, previously undocumented under-count of test coverage.
- **Unclassified**: none — every row above was resolved to either cited evidence (in full or
  partial form) or an explicit, named gap.
