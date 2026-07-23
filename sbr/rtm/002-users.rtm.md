# Requirements Traceability Matrix — Users

- **Source spec**: `specs/002-users/spec.md`
- **Tier mapping in effect**: `sbr/README.md` → "This repo's adapter" (L1=`tests/unit`,
  L2=`tests/integration`, L3=`tests/e2e`, Grounding=`tests/smoke`+`tests/e2e` journeys)
- **Generated**: 2026-07-22 by `speckit-sbr-audit`
- **Mode**: retrofit audit (as-built) — see project constitution Principle I

| FR-ID | Requirement Summary | L1 Unit | L2 Integration | L3 E2E | Grounding (Smoke) | Boundary Status | Notes |
|---|---|---|---|---|---|---|---|
| FR-001 | `GetUser` MUST return the caller's own profile (identity from JWT) joined with every org they belong to, including per-org role and balance. | `TestUserService_GetUserProfile > "Returns each org membership with its role and balance intact"` (`tests/unit/user_service_test.go`) | — | `TestUserService_E2E > "GetUser Profile"` (`tests/e2e/user_test.go:23`) — now also asserts `org.UserRole` (`MEMBER`/`ADMIN`) per org, not just `UserBalance` | `TestSmoke... > "UsersTableReachable_via_Login"` (`tests/smoke/smoke_test.go:142`) | **Complete** | **Fixed 2026-07-23 (SBR remediation Phase 3).** The e2e gap (role field previously unasserted) and the total absence of a unit test for `userService.GetUserProfile` are both closed. No bug found — production behavior matched the spec exactly. |
| FR-002 | `GetUser` MUST NOT fail outright when one membership's org record can't be loaded — MUST return the remaining memberships. | `TestUserService_GetUserProfile > "Skips a membership whose org record can't be loaded, returning the rest"` (`tests/unit/user_service_test.go`) | — | — | — | **Complete** | **Fixed 2026-07-23 (SBR remediation Phase 3).** Confirms a single failed org lookup among 3 memberships is skipped, not fatal — the other 2 orgs are still returned. No bug found. |
| FR-003 | `UpdateProfile` MUST overwrite `name`, `email`, `phone`, `avatar_url` on the caller's row from the request, with no partial-update behavior. | `TestUserService_UpdateProfile > "Blanking a field clears it rather than preserving the prior value"` (`tests/unit/user_service_test.go`) | `TestUserRepository_Integration > "Update"` (`tests/integration/user_test.go:61`) — repo-level only, changes a single field | `TestUserService_E2E > "UpdateProfile"` (`tests/e2e/user_test.go:65`) — happy-path overwrite | — | **Complete** | **Fixed 2026-07-23 (SBR remediation Phase 3).** Directly regression-locks the no-partial-update semantics: blanking `phone`/`avatar_url` in the request clears them rather than preserving the prior stored value. No bug found. |
| FR-004 | `UpdateProfile` MUST reject a write that would violate the `users.email` `UNIQUE` constraint. | — | — | `TestUserService_E2E > "UpdateProfile Email Uniqueness"` (`tests/e2e/user_test.go:115`) — creates two users, attempts to update user2's email to user1's exact-case email, asserts `assert.Error(t, err)` | — | **Gap — L1, L2** | The rejection itself (the MUST clause) is verified at L3, but only by a generic `assert.Error` — no unit test isolates the service/repo behavior, and no integration test creates a duplicate-email conflict directly against Postgres. (The `docs/design` gap around case-sensitivity vs. `Signup`'s case-insensitive check — Known Discrepancy 2 — has zero test coverage at any tier, confirmed by grep; that gap is outside FR-004's own wording but adjacent.) |
| FR-005 | As-built, `UpdateProfile` MUST NOT be assumed to validate email format or reject empty `name`/`phone`/`email` beyond DB `NOT NULL` constraints. | — | — | — | — | **Gap — all tiers** | Zero test anywhere calls `UpdateProfile` with an empty/malformed `email`, empty `name`, or empty `phone` (confirmed by grep across `tests/e2e/user_test.go` and `tests/integration` — no matches). This is a "negative"/absence requirement: the only evidence that would satisfy it is a regression-lock test proving malformed input is currently accepted. None exists, so this documented gap in application behavior could be silently closed (someone adds validation) or silently widened without any test failing either way. Matches spec.md's own Coverage Baseline, which lists this as "Not covered anywhere." |

## Summary

- **5 FR-IDs audited, 3 fully `Complete`** (FR-001, FR-002, FR-003 closed 2026-07-23 as SBR
  remediation Phase 3).
- **Remaining gaps**: FR-004 (rejection proven only via generic `assert.Error`), FR-005
  (documented absence of input validation) — tracked in `sbr/remediation-plan.md` Phase 4.
- **Phase 3 outcome**: FR-001's per-org role field (previously unasserted at e2e, and with zero
  unit coverage of `userService.GetUserProfile`) and FR-002's partial-org-lookup resilience
  closed with new unit tests plus an e2e assertion strengthening. FR-003's no-partial-update
  semantics (blanking a field clears it rather than preserving the prior value) closed with a
  new unit test for `userService.UpdateProfile` — the first unit test ever written for this
  service. No bugs found in any of the three; all were purely coverage gaps.
- **Unclassified**: none — every row above was resolved to either cited evidence (in full or
  partial form) or an explicit, named gap.
