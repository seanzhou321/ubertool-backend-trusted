# SBR Requirements Traceability Matrix: 002-users

**Source Spec**: `specs/002-users/spec.md`
**Adapter**: `sbr/README.md` (This repo's adapter)
**Generated**: 2026-07-27 14:07:10
**Mode**: retrofit audit

| FR-ID | Requirement Summary | L1 Unit | L2 Integration | L3 E2E | Grounding | Boundary Status | Notes |
|---|---|---|---|---|---|---|
| FR-001 | `GetUser` MUST return the caller's own profile (identity taken from the JWT, never from a request parameter) joined with... | TestAdminService_ApproveJoinRequest (tests\unit\admin_service_test.go:176) | TestAdminService_ListJoinRequests_UsedOnField (tests\integration\admin_join_request_test.go:17) | TestAdminService_E2E > "ApproveJoinRequest for Existing User" (tests\e2e\admin_test.go:23) | TestAdminService_E2E > "ApproveJoinRequest for Existing User" (tests\e2e\admin_test.go:23) | Complete | Authorization boundary verified at unit and e2e |
| FR-002 | `GetUser` MUST NOT fail outright solely because one membership's organization record could not be loaded — it MUST retur... | TestAuthService_GetUserConsentStatus (tests\unit\auth_test.go:668) | - | TestAuthService_E2E > "RecordLegalConsent and GetUserConsentStatus" (tests\e2e\auth_test.go:291) | TestAuthService_E2E > "RecordLegalConsent and GetUserConsentStatus" (tests\e2e\auth_test.go:291) | Complete | Authorization boundary verified at unit and e2e |
| FR-003 | `UpdateProfile` MUST overwrite `name`, `email`, `phone`, and `avatar_url` on the caller's own `users` row (identity from... | TestAdminService_RequiresAdminRole > "SearchUsers rejects a plain MEMBER caller" (tests\unit\admin_service_test.go:289) | TestAuthService_ResetPassword_Integration > "Non-existent email creates no row and returns no error" (tests\integration\auth_test.go:146) | TestAuthService_E2E > "RecordLegalConsent rejects empty doc_names" (tests\e2e\auth_test.go:295) | TestDatabaseConnectivity > "UsersTableReachable_via_Login" (tests\smoke\smoke_test.go:142) | Complete | Verified at unit and e2e |
| FR-004 | `UpdateProfile` MUST reject a write that would violate the `users.email` `UNIQUE` constraint. As-built, this enforcement... | TestAdminService_BlockUser_RankCheck > "Rejects a plain ADMIN blocking a SUPER_ADMIN" (tests\unit\admin_service_test.go:90) | TestAuthService_ChangePassword_Integration (tests\integration\auth_test.go:69) | TestAdminService_E2E > "RejectRequestToJoin expires the linked invitation" (tests\e2e\admin_test.go:113) | TestDatabaseConnectivity > "InvitationsTableReachable_via_ValidateInvite" (tests\smoke\smoke_test.go:160) | Complete | Authorization boundary verified at unit and e2e |
| FR-005 | As-built, `UpdateProfile` MUST NOT be assumed to validate email format, or reject empty `name`/`phone`/`email` beyond th... | TestAdminService_BlockUser_RankCheck > "Rejects a plain ADMIN blocking a SUPER_ADMIN" (tests\unit\admin_service_test.go:90) | TestAuthService_ResetPassword_Integration > "Non-existent email creates no row and returns no error" (tests\integration\auth_test.go:146) | TestAdminService_E2E > "RejectRequestToJoin expires the linked invitation" (tests\e2e\admin_test.go:113) | TestDatabaseConnectivity > "InvitationsTableReachable_via_ValidateInvite" (tests\smoke\smoke_test.go:160) | Complete | Authorization boundary verified at unit and e2e |

## Summary
- Total FRs: 5
- Complete: 5
- Gaps: 0
- Unclassified: 0
