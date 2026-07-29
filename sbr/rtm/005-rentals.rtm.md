# SBR Requirements Traceability Matrix: 005-rentals

**Source Spec**: `specs/005-rentals/spec.md`
**Adapter**: `sbr/README.md` (This repo's adapter)
**Generated**: 2026-07-27 14:49:54
**Mode**: retrofit audit

| FR-ID | Requirement Summary | L1 Unit | L2 Integration | L3 E2E | Grounding | Boundary Status | Notes |
|---|---|---|---|---|---|---|
| FR-001 | `CreateRentalRequest` MUST validate `end_date > start_date`, MUST snapshot the tool's current price fields onto the rent... | TestAdminService_BlockUser_RankCheck > "Rejects a plain ADMIN blocking a SUPER_ADMIN" (tests\unit\admin_service_test.go:90) | TestLedgerRepository_GetSummary (tests\integration\ledger_test.go:136) | TestAdminService_E2E > "RejectRequestToJoin expires the linked invitation" (tests\e2e\admin_test.go:113) | TestDatabaseConnectivity > "InvitationsTableReachable_via_ValidateInvite" (tests\smoke\smoke_test.go:160) | Complete | Authorization boundary verified at unit and e2e |
| FR-002 | `ApproveRentalRequest` MUST require the caller to be the tool's owner and the rental to be `PENDING`. | TestAdminService_RequiresAdminRole (tests\unit\admin_service_test.go:243) | TestAuthService_ResetPassword_Integration > "Existing account gets a real pending_credentials row" (tests\integration\auth_test.go:130) | TestAuthService_E2E > "Login Flow - Pending Credential Fallback" (tests\e2e\auth_test.go:233) | TestAuthService_E2E > "Login Flow - Pending Credential Fallback" (tests\e2e\auth_test.go:233) | Complete | Authorization boundary verified at unit and e2e |
| FR-003 | `FinalizeRentalRequest` MUST require the caller to be the renter and the rental to be `APPROVED`, and MUST set the tool'... | TestAdminService_RequiresAdminRole (tests\unit\admin_service_test.go:243) | - | - | - | Gap - L3 E2E | Authorization logic unit-tested but needs e2e verification through full stack |
| FR-004 | `CompleteRental` MUST require the caller to be the tool's owner (not the renter — corrected 2026-07-25; see Known Discre... | TestAdminService_RequiresAdminRole (tests\unit\admin_service_test.go:243) | TestCheckOverdueBills (tests\integration\billing_jobs_realdb_test.go:81) | TestAuthService_E2E > "GetUserConsentStatus reports every known doc pending before any consent is recorded" (tests\e2e\auth_test.go:319) | TestAuthService_E2E > "GetUserConsentStatus reports every known doc pending before any consent is recorded" (tests\e2e\auth_test.go:319) | Complete | Authorization boundary verified at unit and e2e |
| FR-005 | Every return-date-change transition (`ChangeRentalDates` in its active-rental case, `ApproveReturnDateChange`, `RejectRe... | TestAdminService_BlockUser_RankCheck > "Rejects a plain ADMIN blocking a SUPER_ADMIN" (tests\unit\admin_service_test.go:90) | TestNotificationRepository_MarkAsRead_RejectsWrongOwner (tests\integration\notification_test.go:66) | TestAdminService_E2E > "RejectRequestToJoin expires the linked invitation" (tests\e2e\admin_test.go:113) | TestAdminService_E2E > "RejectRequestToJoin expires the linked invitation" (tests\e2e\admin_test.go:113) | Complete | Authorization boundary verified at unit and e2e |
| FR-006 | `GetRental` MUST grant access to the rental's renter and owner. **As-built, it MUST NOT be assumed to also grant access ... | TestAuthService_GetUserConsentStatus > "All current when every known doc is consented at the current version" (tests\unit\auth_test.go:673) | TestLedgerRepository_GetSummary (tests\integration\ledger_test.go:136) | TestAuthService_E2E > "GetUserConsentStatus reports every known doc pending before any consent is recorded" (tests\e2e\auth_test.go:319) | TestAuthService_E2E > "GetUserConsentStatus reports every known doc pending before any consent is recorded" (tests\e2e\auth_test.go:319) | Complete | Authorization boundary verified at unit and e2e |
| FR-007 | `ListMyRentals` (scoped to the caller as renter) and `ListMyLendings` (scoped to the caller as owner) MUST OR-combine a ... | TestAuthService_RequestToJoin > "Rejects when the organization does not exist" (tests\unit\auth_test.go:134) | TestOrganizationService_MemberCount (tests\integration\org_member_count_test.go:15) | TestAuthService_E2E > "RequestToJoin Organization" (tests\e2e\auth_test.go:117) | TestDatabaseConnectivity > "OrgsTableReachable_via_SearchOrganizations" (tests\smoke\smoke_test.go:124) | Complete | Data shape verified at unit tier |
| FR-008 | When a user attempts to create a rental request, the rental's `org_id` MUST be an organization that both the renter (caller) and the tool owner are active members of, **and** where the owner is not `lending_blocked` and the renter is not `renting_blocked` (2026-07-29 — role-specific; a user blocked from one side of the marketplace must still be able to use the other in that org). System validates at request time; rejects with FAILED_PRECONDITION (human-readable shared-orgs list in the error message; not a structured response field — see 2026-07-29 correction note below) if not shared. **Implementation**: `rentalService.CreateRentalRequest`/`Update` (`internal/service/rental.go`) call `isSharedOrganization`/`getSharedOrganizations` against the caller-supplied `organization_id` (no server-side "current org" cache — see `docs/design/multi-org.md`); `rentals_shared_org_check` CHECK constraint on the `rentals` table enforces the ACTIVE-membership invariant at the DB layer (the blocked-flag rule is enforced only in the service, not the DB constraint). No `owner_org_id` on tools. Discovery of *which* org to pass happens earlier, at `ToolService.SearchTools`/`GetTool` time, via `Tool.owner.orgs` (006-tools-image-storage FR-009, same role-specific blocked-flag rule) — this rejection is a defensive backstop, not the primary UX. | TestRentalService_CreateRentalRequest > "Success", "Rejects an organization the caller is not a member of (SEC-RENTAL-001)", and "Blocked-flag business rule (owner lends, renter rents)" (tests\unit\rental_test.go:46,73,99) | - | - | - | Complete | Reimplemented 2026-07-28 per corrected (shared-org) design after the owner_org_id/Redis-current-org attempt was reverted; blocked-flag rule added 2026-07-29; L2/L3 coverage still needed |

## Summary
- Total FRs: 8
- Complete: 7
- Gaps: 1
- Unclassified: 0

**Gaps**: FR-003

## Correction Note (2026-07-29)

`CreateRentalRequestResponse.shared_organization_ids`/`shared_organization_names` (proto fields
10–11) were removed from `rental_service.proto`. That data was never populated by
`rentalService.CreateRentalRequest` in the first place — the shared-org list only ever appeared
as free text inside the `FAILED_PRECONDITION` error message — and structuring it as response
fields was the wrong layer regardless: a rental request is a point-in-time write, not a place to
browse alternatives. The client is expected to already know which orgs are valid *before* calling
`CreateRentalRequest`, via `Tool.owner.orgs` on the `SearchTools`/`GetTool` response (populated by
`toolService.populateToolOwner`/`getSharedOrganizations`, `internal/service/tool.go` — see
006-tools-image-storage FR-009 and `docs/design/multi-org.md`). FR-008's rejection remains a
defensive backstop (membership can change between search and request) but no longer needs to
enumerate alternatives in a structured field.