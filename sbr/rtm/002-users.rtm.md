# Requirements Traceability Matrix — Users

- **Source spec**: `specs/002-users/spec.md`
- **Tier mapping in effect**: `sbr/README.md` → "This repo's adapter" (L1=`tests/unit`,
  L2=`tests/integration`, L3=`tests/e2e`, Grounding=`tests/smoke`+`tests/e2e` journeys)
- **Generated**: 2026-07-22 by `speckit-sbr-audit`; updated 2026-07-23 (Phase 4 re-audit)
- **Mode**: retrofit audit (as-built) — see project constitution Principle I

| FR-ID | Requirement Summary | L1 Unit | L2 Integration | L3 E2E | Grounding (Smoke) | Boundary Status | Notes |
|---|---|---|---|---|---|---|---|
| FR-001 | `GetUser` MUST return the caller's own profile (identity from JWT) joined with every org they belong to, including per-org role and balance. | `TestUserService_GetUserProfile > "Returns each org membership with its role and balance intact"` (`tests/unit/user_service_test.go`) | — | `TestUserService_E2E > "GetUser Profile"` (`tests/e2e/user_test.go:23`) — now also asserts `org.UserRole` (`MEMBER`/`ADMIN`) per org, not just `UserBalance` | `TestSmoke... > "UsersTableReachable_via_Login"` (`tests/smoke/smoke_test.go:142`) | **Complete** | **Fixed 2026-07-23 (SBR remediation Phase 3).** The e2e gap (role field previously unasserted) and the total absence of a unit test for `userService.GetUserProfile` are both closed. No bug found — production behavior matched the spec exactly. |
| FR-002 | `GetUser` MUST NOT fail outright when one membership's org record can't be loaded — MUST return the remaining memberships. | `TestUserService_GetUserProfile > "Skips a membership whose org record can't be loaded, returning the rest"` (`tests/unit/user_service_test.go`) | — | — | — | **Complete** | **Fixed 2026-07-23 (SBR remediation Phase 3).** Confirms a single failed org lookup among 3 memberships is skipped, not fatal — the other 2 orgs are still returned. No bug found. |
| FR-003 | `UpdateProfile` MUST overwrite `name`, `email`, `phone`, `avatar_url` on the caller's row from the request, with no partial-update behavior. | `TestUserService_UpdateProfile > "Blanking a field clears it rather than preserving the prior value"` (`tests/unit/user_service_test.go`) | `TestUserRepository_Integration > "Update"` (`tests/integration/user_test.go:61`) — repo-level only, changes a single field | `TestUserService_E2E > "UpdateProfile"` (`tests/e2e/user_test.go:65`) — happy-path overwrite | — | **Complete** | **Fixed 2026-07-23 (SBR remediation Phase 3).** Directly regression-locks the no-partial-update semantics: blanking `phone`/`avatar_url` in the request clears them rather than preserving the prior stored value. No bug found. |
| FR-004 | `UpdateProfile` MUST reject a write that would violate the `users.email` `UNIQUE` constraint. | `TestUserService_UpdateProfile > "Propagates a repository rejection (e.g. duplicate email)"` (`tests/unit/user_service_test.go`) — confirms the service surfaces a repository-level rejection rather than swallowing it | `TestUserRepository_Integration > "Update rejects a duplicate email"` (`tests/integration/user_test.go`) — real Postgres, creates two users and asserts the real `UNIQUE` constraint violation on `Update`, plus that neither row is left mutated | `TestUserService_E2E > "UpdateProfile Email Uniqueness"` (`tests/e2e/user_test.go:115`) | — | **Complete** | **Fixed 2026-07-23 (SBR remediation Phase 4).** No bug found — the constraint was already correctly enforced; this closed the L1/L2 coverage gap. |
| FR-005 | As-built, `UpdateProfile` MUST NOT be assumed to validate email format or reject empty `name`/`phone`/`email` beyond DB `NOT NULL` constraints. | — | — | `TestUserService_E2E > "UpdateProfile accepts malformed/empty input (documents absence of validation)"` (`tests/e2e/user_test.go`) — submits an empty `name`, empty `phone`, and a malformed (non-`@`) `email`, asserts the call succeeds and the DB stores exactly those values | — | **Complete** | **Fixed 2026-07-23 (SBR remediation Phase 4).** This is a regression-lock on the as-built lenient (non-)validation behavior, not a design fix — if input validation is ever added, this test will fail, correctly signaling the documented behavior changed on purpose. No bug found (behavior confirmed exactly as spec.md's Coverage Baseline described). |

## Summary

- **5 FR-IDs audited, all 5 fully `Complete`** (FR-001, FR-002, FR-003 closed 2026-07-23 as SBR
  remediation Phase 3; FR-004, FR-005 closed 2026-07-23 as Phase 4).
- **Remaining gaps**: none.
- **Phase 3 outcome**: FR-001's per-org role field (previously unasserted at e2e, and with zero
  unit coverage of `userService.GetUserProfile`) and FR-002's partial-org-lookup resilience
  closed with new unit tests plus an e2e assertion strengthening. FR-003's no-partial-update
  semantics (blanking a field clears it rather than preserving the prior value) closed with a
  new unit test for `userService.UpdateProfile` — the first unit test ever written for this
  service. No bugs found in any of the three; all were purely coverage gaps.
- **Phase 4 outcome**: FR-004's email-uniqueness rejection closed at L1 (service propagates the
  rejection) and L2 (a real Postgres `UNIQUE` violation against the repository, replacing the
  previous generic e2e-only `assert.Error`). FR-005's documented absence of input validation
  closed with an e2e regression-lock proving the current lenient behavior. No bugs found in
  either.
- **Unclassified**: none — every row above was resolved to either cited evidence (in full or
  partial form) or an explicit, named gap.
