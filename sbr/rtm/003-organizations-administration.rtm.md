# Requirements Traceability Matrix — Organizations & Administration

- **Source spec**: `specs/003-organizations-administration/spec.md`
- **Tier mapping in effect**: `sbr/README.md` → "This repo's adapter" (L1=`tests/unit`,
  L2=`tests/integration`, L3=`tests/e2e`, Grounding=`tests/smoke`+`tests/e2e` journeys)
- **Generated**: 2026-07-22 by `speckit-sbr-audit`
- **Mode**: retrofit audit (as-built) — see project constitution Principle I

| FR-ID | Requirement Summary | L1 Unit | L2 Integration | L3 E2E | Grounding (Smoke) | Boundary Status | Notes |
|---|---|---|---|---|---|---|---|
| FR-001 | `CreateOrganization` requires no pre-existing authorization; caller becomes `SUPER_ADMIN`. | `TestOrganizationService_CreateOrganization` (`tests/unit/org_service_test.go`) | — | `TestOrganizationService_E2E > "CreateOrganization with SUPER_ADMIN Assignment"` (`tests/e2e/org_test.go:23`) | — | **Complete** | **Fixed 2026-07-23 (SBR remediation Phase 3).** Regression-locks both halves: no pre-existing authorization required, and the caller is added as `SUPER_ADMIN`. No bug found. |
| FR-002 | `UpdateOrganization` must reject non-members, require `ADMIN`/`SUPER_ADMIN` for any change, and require `SUPER_ADMIN` specifically for price-field changes (submitted `0` = no change). | `TestOrganizationService_UpdateOrganization` (`tests/unit/org_service_test.go:31`) — 7 subtests: SUPER_ADMIN success, ADMIN non-price-field success, non-member reject, plain-MEMBER reject, ADMIN-attempts-threshold reject (both fields), "0 = no change" semantics | — | — | — | **Complete** | **Fixed 2026-07-22.** Previously only the SUPER_ADMIN success path was tested, contradicting spec.md's SC-002 claim of "explicit regression tests locking in its current correct behavior." All 7 new subtests pass — no bug found, this was purely a coverage gap. SC-002 corrected in spec.md to reflect actual coverage. |
| FR-003 | `UpdateOrganization` broadcasts (notification + email + push) to every active member when a price threshold actually changes. | `TestOrganizationService_UpdateOrganization_ThresholdBroadcast` (`tests/unit/org_service_test.go`) — 2 subtests: broadcasts (in-app notification + email + FCM multicast, excluding blocked members) when the settlement threshold changes, and does NOT broadcast when no threshold field actually changes | — | — | — | **Complete** | **Fixed 2026-07-23 (SBR remediation Phase 2).** The broadcast runs in a background goroutine; the test synchronizes on the FCM multicast call via a channel. No bug found. |
| FR-004 | `JoinOrganizationWithInvite` validates invite-email ownership, rejects if already a member, notifies org admins on success. | `TestOrganizationService_JoinOrganizationWithInvite` (`tests/unit/org_service_test.go`) — 4 subtests: success adds caller as MEMBER, rejects already-a-member, rejects invitation issued to a different email, rejects an already-used invitation | — | — | — | **Complete** | **Fixed 2026-07-23 (SBR remediation Phase 2).** Previously only a mock interface stub existed. No bug found. |
| FR-005 | Every `AdminService` RPC must require `ADMIN`/`SUPER_ADMIN` membership in the target org, checked before any other work. | `TestAdminService_RequiresAdminRole` (`tests/unit/admin_service_test.go:151`) — 8 subtests, one per RPC's rejection path, plus one confirming `ADMIN`/`SUPER_ADMIN` acceptance | — | `TestAdminService_E2E` (`tests/e2e/admin_test.go:13`) — admin-caller **happy paths only**; no rejection-path subtest | — | **Gap — L3 (known)** | L1 is thorough and per-RPC — this is the tier that most directly verifies the MUST clause, and it exists because a real gap here (`Known Discrepancy 1`) was found and fixed on 2026-07-22. The L3 gap is **already self-documented in spec.md's own SC-001** ("an equivalent e2e-level assertion... still can't prove the rejection path end-to-end") — this RTM agrees with, not newly discovers, that status. L2 not flagged: an authorization gate proven at L1 against a real repo interface gains little additional confidence from a real-DB integration test here. |
| FR-006 | `RejectRequestToJoin` expires any invitation already linked to the rejected join request. | `TestAdminService_RejectJoinRequest_ExpiresLinkedInvitation` (`tests/unit/admin_service_test.go`) — 2 subtests: expires a linked not-yet-used invitation, does not touch an already-used invitation | — | — | — | **Complete** | **Fixed 2026-07-23 (SBR remediation Phase 3).** No bug found. |

## Summary

- **6 FR-IDs audited, 5 fully `Complete`** (FR-002 closed 2026-07-22 as SBR remediation Phase 1;
  FR-003, FR-004 closed 2026-07-23 as Phase 2; FR-001, FR-006 closed 2026-07-23 as Phase 3).
- **Remaining gaps**: FR-005 (known/tracked L3 gap, self-documented in spec.md's own SC-001) —
  tracked in `sbr/remediation-plan.md` Phase 4.
- **Phase 1 outcome**: FR-002's SC-002 spec-vs-test contradiction resolved — 7 new subtests
  added, no bug found (production behavior already matched the spec), spec.md corrected to
  reflect actual coverage.
- **Phase 2 outcome**: FR-003's threshold-change broadcast (notification + email + FCM
  multicast, run in a background goroutine) unit-tested; FR-004's `JoinOrganizationWithInvite`
  unit-tested (success + 3 reject clauses). No bugs found in either.
- **Phase 3 outcome**: FR-001's no-pre-authorization + SUPER_ADMIN-assignment guarantee and
  FR-006's invitation-expiry side effect on `RejectRequestToJoin` both unit-tested. No bugs
  found in either.
- **Unclassified**: none — every row above was resolved to either cited evidence or an
  explicit, named gap.
