# Requirements Traceability Matrix — Organizations & Administration

- **Source spec**: `specs/003-organizations-administration/spec.md`
- **Tier mapping in effect**: `sbr/README.md` → "This repo's adapter" (L1=`tests/unit`,
  L2=`tests/integration`, L3=`tests/e2e`, Grounding=`tests/smoke`+`tests/e2e` journeys)
- **Generated**: 2026-07-22 by `speckit-sbr-audit`; updated 2026-07-23 (Phase 4 re-audit)
- **Mode**: retrofit audit (as-built) — see project constitution Principle I

| FR-ID | Requirement Summary | L1 Unit | L2 Integration | L3 E2E | Grounding (Smoke) | Boundary Status | Notes |
|---|---|---|---|---|---|---|---|
| FR-001 | `CreateOrganization` requires no pre-existing authorization; caller becomes `SUPER_ADMIN`. | `TestOrganizationService_CreateOrganization` (`tests/unit/org_service_test.go`) | **Intentionally not separately tested** — see Notes | `TestOrganizationService_E2E > "CreateOrganization with SUPER_ADMIN Assignment"` (`tests/e2e/org_test.go:23`) | — | **Complete** | **Fixed 2026-07-23 (SBR remediation Phase 3).** Regression-locks both halves: no pre-existing authorization required, and the caller is added as `SUPER_ADMIN`. No bug found. L2 not added (2026-07-23 hardening pass): the e2e test already asserts the real `users_orgs.role` row directly against Postgres — L3 subsumes L2 here. |
| FR-002 | `UpdateOrganization` must reject non-members, require `ADMIN`/`SUPER_ADMIN` for any change, and require `SUPER_ADMIN` specifically for price-field changes (submitted `0` = no change). | `TestOrganizationService_UpdateOrganization` (`tests/unit/org_service_test.go:31`) — 7 subtests: SUPER_ADMIN success, ADMIN non-price-field success, non-member reject, plain-MEMBER reject, ADMIN-attempts-threshold reject (both fields), "0 = no change" semantics | `TestOrganizationService_E2E > "UpdateOrganization by SUPER_ADMIN updates fields and price thresholds"` (`tests/e2e/org_test.go`) | (same test, at L3 — this is a real gRPC/DB call, so it satisfies both tiers at once) | — | **Complete** | **Fixed 2026-07-22 (L1).** Previously only the SUPER_ADMIN success path was tested, contradicting spec.md's SC-002 claim of "explicit regression tests locking in its current correct behavior." All 7 new subtests pass — no bug found, this was purely a coverage gap. SC-002 corrected in spec.md to reflect actual coverage. **L2/L3 added 2026-07-23 (hardening pass):** prior to this, `grep -r UpdateOrganization tests/e2e tests/integration` returned nothing — this RPC had never once been called through the real gRPC handler against a live DB. New e2e test confirms all 6 updatable fields (including both price thresholds) persist correctly. No bug found. |
| FR-003 | `UpdateOrganization` broadcasts (notification + email + push) to every active member when a price threshold actually changes. | `TestOrganizationService_UpdateOrganization_ThresholdBroadcast` (`tests/unit/org_service_test.go`) — 2 subtests: broadcasts (in-app notification + email + FCM multicast, excluding blocked members) when the settlement threshold changes, and does NOT broadcast when no threshold field actually changes | (see FR-002's e2e test, at L3) | `TestOrganizationService_E2E > "UpdateOrganization by SUPER_ADMIN updates fields and price thresholds"` (`tests/e2e/org_test.go`) — polls for the broadcast's in-app notification to reach an active member after a real threshold change, since the broadcast runs in a background goroutine | — | **Complete** | **Fixed 2026-07-23 (SBR remediation Phase 2, L1; hardening pass, L3).** The broadcast runs in a background goroutine; the L1 test synchronizes on the FCM multicast call via a channel. The new e2e assertion confirms the same broadcast actually reaches a real member's `notifications` row through the full stack. No bug found in either pass. |
| FR-004 | `JoinOrganizationWithInvite` validates invite-email ownership, rejects if already a member, notifies org admins on success. | `TestOrganizationService_JoinOrganizationWithInvite` (`tests/unit/org_service_test.go`) — 4 subtests: success adds caller as MEMBER, rejects already-a-member, rejects invitation issued to a different email, rejects an already-used invitation | **Intentionally not separately tested** — see Notes | `TestOrganizationService_E2E > "JoinOrganizationWithInvite"` (`tests/e2e/org_test.go`) — a real invitation, joined through the real gRPC handler against a live DB; asserts the resulting `users_orgs` membership row and the invitation's `used_on` stamp | — | **Complete** | **Fixed 2026-07-23 (SBR remediation Phase 2, L1).** Previously only a mock interface stub existed. No bug found. **L3 added 2026-07-23 (hardening pass):** prior to this, `grep -r JoinOrganizationWithInvite tests/e2e tests/integration` returned nothing. L2 not added: the new e2e test already asserts the real `users_orgs`/`invitations` rows directly — L3 subsumes L2 here. |
| FR-005 | Every `AdminService` RPC must require `ADMIN`/`SUPER_ADMIN` membership in the target org, checked before any other work. | `TestAdminService_RequiresAdminRole` (`tests/unit/admin_service_test.go:151`) — 8 subtests, one per RPC's rejection path, plus one confirming `ADMIN`/`SUPER_ADMIN` acceptance | — | `TestAdminService_E2E > "AdminBlockUserAccount rejects a non-admin caller"` (`tests/e2e/admin_test.go`) — a plain MEMBER caller through the real gRPC handler against a live DB, confirming the rejected call had no effect; other 7 RPCs remain e2e-happy-path-only, per L1's per-RPC coverage already being the primary verification tier | — | **Complete** | **Fixed 2026-07-23 (SBR remediation Phase 4).** L1 is thorough and per-RPC — this is the tier that most directly verifies the MUST clause, and it exists because a real gap here (`Known Discrepancy 1`) was found and fixed on 2026-07-22. The previously-self-documented L3 gap (spec.md's own SC-001) is now reinforced with a representative e2e rejection test; L2 not flagged, consistent with the same reasoning as before — an authorization gate proven at L1 against a real repo interface gains little additional confidence from a real-DB integration test here. No bug found. |
| FR-006 | `RejectRequestToJoin` expires any invitation already linked to the rejected join request. | `TestAdminService_RejectJoinRequest_ExpiresLinkedInvitation` (`tests/unit/admin_service_test.go`) — 2 subtests: expires a linked not-yet-used invitation, does not touch an already-used invitation | **Intentionally not separately tested** — see Notes | `TestAdminService_E2E > "RejectRequestToJoin expires the linked invitation"` (`tests/e2e/admin_test.go`) — a real join request + linked invitation, rejected through the real gRPC handler against a live DB; asserts the join request's `status` and the invitation's `expires_on` | — | **Complete** | **Fixed 2026-07-23 (SBR remediation Phase 3, L1).** No bug found. **L3 added 2026-07-23 (hardening pass):** prior to this, `grep -r RejectRequestToJoin tests/e2e tests/integration` returned nothing. L2 not added: the new e2e test already asserts the real `join_requests`/`invitations` rows directly — L3 subsumes L2 here. |

## Summary

- **6 FR-IDs audited, all 6 fully `Complete`** (FR-002 closed 2026-07-22 as SBR remediation
  Phase 1; FR-003, FR-004 closed 2026-07-23 as Phase 2; FR-001, FR-006 closed 2026-07-23 as
  Phase 3; FR-005 closed 2026-07-23 as Phase 4).
- **Remaining gaps**: none.
- **Phase 1 outcome**: FR-002's SC-002 spec-vs-test contradiction resolved — 7 new subtests
  added, no bug found (production behavior already matched the spec), spec.md corrected to
  reflect actual coverage.
- **Phase 2 outcome**: FR-003's threshold-change broadcast (notification + email + FCM
  multicast, run in a background goroutine) unit-tested; FR-004's `JoinOrganizationWithInvite`
  unit-tested (success + 3 reject clauses). No bugs found in either.
- **Phase 3 outcome**: FR-001's no-pre-authorization + SUPER_ADMIN-assignment guarantee and
  FR-006's invitation-expiry side effect on `RejectRequestToJoin` both unit-tested. No bugs
  found in either.
- **Phase 4 outcome**: FR-005's previously self-documented, known L3 gap (spec.md's own SC-001)
  reinforced with a representative e2e non-admin-rejection test against a real gRPC handler and
  live DB. No bug found; spec.md's SC-001 updated to reflect the added coverage.
- **Phase 4 hardening pass (2026-07-23)**: re-audited every cell for unexplained blanks. Found
  that 3 RPCs — `UpdateOrganization` (FR-002/FR-003), `JoinOrganizationWithInvite` (FR-004), and
  `RejectRequestToJoin` (FR-006) — had thorough L1 coverage but had **never once been called
  through the real gRPC handler against a live database** (`grep -r <rpc-name>
  tests/e2e tests/integration` returned nothing for all three). Closed with 3 new e2e subtests:
  `UpdateOrganization` (asserts all 6 updatable fields persist, including both price thresholds,
  plus polls for FR-003's threshold-change broadcast reaching a real member's `notifications`
  row), `JoinOrganizationWithInvite` (asserts the resulting membership row and invitation
  used-stamp), and `RejectRequestToJoin` (asserts the join request status and the linked
  invitation's expiry). No bugs found in any of the three — production behavior matched the
  spec exactly in every case; these were purely coverage gaps. FR-001's L2 and the 3 new
  e2e-only rows' L2 cells were left undocumented-by-a-separate-test where the new/existing e2e
  test already subsumes what L2 would check (documented per-row above).
- **Unclassified**: none — every row above was resolved to either cited evidence or an
  explicit, named gap.
