# Implementation Plan: Organizations & Administration (As-Built + Multi-Org FR-009/010/011/012)

**Branch**: `004-organizations-administration` | **Date**: 2026-07-27 | **Spec**: `specs/003-organizations-administration/spec.md`

**Input**: Feature specification from `specs/003-organizations-administration/spec.md`

**Note**: This template is filled in by the `/speckit-plan` command; its definition describes the execution workflow.

## Summary

[Extract from feature spec: primary requirement + technical approach from research]

## Technical Context

**Language/Version**: Go 1.22+ (standard library + gRPC)

**Primary Dependencies**: 
- google.golang.org/grpc
- google.golang.org/protobuf
- github.com/jackc/pgx/v5 (PostgreSQL driver)
- github.com/golang-migrate/migrate/v4 (migrations)
- github.com/stretchr/testify (testing)

**Storage**: PostgreSQL (schema: `podman/trusted-group/postgres/ubertool_schema_trusted.sql`)

**Testing**: Go standard library `testing` + `testify`; tiers per Constitution Principle IV:
- `tests/unit` — pure logic, mocks only
- `tests/integration` — service + repository + real DB
- `tests/e2e` — full gRPC surface via generated client
- `tests/smoke` — deploy sanity checks

**Target Platform**: Linux server (Podman on Windows 11 locally, EC2 micro in AWS)

**Project Type**: gRPC microservice (part of ubertool-backend-trusted monorepo)

**Performance Goals**: 
- P95 < 200ms for CreateOrganization, GetOrganization, ListMyOrganizations
- P95 < 500ms for AdminService RPCs (Approve/Reject/ListJoinRequests)

**Constraints**: 
- Must deploy to both Podman (local) and EC2 (prod) per Constitution Principle V
- Proto-first API (Principle VI): changes start in `api/proto/.../organization_service.proto` + `admin_service.proto`
- Layering: handler → service → repository → DB (Architecture Constraints)
- Domain constants for status-like fields (Principle II)
- Push notification pattern for mutations (Principle III)

**Scale/Scope**: 
- 8 RPCs: CreateOrganization, UpdateOrganization, GetOrganization, ListMyOrganizations, JoinOrganizationWithInvite, ApproveRequestToJoin, RejectRequestToJoin, SendInvitation, AdminBlockUserAccount, ListJoinRequests
- Tables: `orgs`, `users_orgs`, `join_requests`, `invitations`
- **Multi-Org Gaps (new FRs)**: FR-009 (multi-org membership), FR-010 (org context), FR-011 (cross-org search), FR-012 (rental context switch)

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Check | Result |
|-----------|-------|--------|
| I. Reconcile Discrepancies Among Spec, RTM, and Code | Plan derived from `internal/service/org.go`, `internal/service/admin.go`, `internal/api/grpc/org.go`, `internal/api/grpc/admin.go`, `api/proto/.../organization_service.proto`, `admin_service.proto`, schema SQL | ✅ PASS |
| II. Domain Model Constraint | `users_orgs.role`/`status` use domain constants; new FR-009 `balance_cents` per-membership | ✅ PASS |
| III. Push Notification Pattern | Mutations (CreateOrg, JoinOrg, Approve/RejectRequest, SendInvite, BlockUser) → notifications + email + push | ✅ PASS |
| IV. Layered Testing Discipline | Tests map to existing `tests/unit`, `tests/integration`, `tests/e2e`, `tests/smoke` tiers | ✅ PASS |
| V. Deployment Parity | Schema changes work on both Podman and EC2 PostgreSQL | ✅ PASS |
| VI. Proto-First API Contract | Any API change starts with proto regeneration | ✅ PASS |

**Gate Status**: ✅ All checks pass — proceed to Phase 0

## Project Structure

### Documentation (this feature)

```text
specs/003-organizations-administration/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output
└── tasks.md             # Phase 2 output (/speckit-tasks command)
```

### Source Code (repository root)

```text
internal/
├── api/grpc/
│   ├── org.go                    # OrganizationService handlers
│   └── admin.go                  # AdminService handlers
├── service/
│   ├── org.go                    # OrganizationService business logic
│   └── admin.go                  # AdminService business logic
├── repository/postgres/
│   ├── org.go                    # OrganizationRepository
│   └── admin.go                  # AdminRepository
├── domain/
│   ├── org.go                    # Domain types & constants
│   └── admin.go                  # Domain types & constants
config/
api/proto/ubertool_trusted_backend/v1/
├── organization_service.proto    # OrgService contract
└── admin_service.proto           # AdminService contract
podman/trusted-group/postgres/ubertool_schema_trusted.sql   # Schema

cmd/
├── server/main.go                # gRPC server entrypoint
└── cronjob/main.go               # Background jobs (N/A for Org/Admin)

### Test Structure (existing tiers)

tests/
├── unit/org_service_test.go           # Pure logic tests
├── unit/admin_service_test.go
├── integration/org_test.go            # Service + repo + real DB
├── integration/admin_test.go
├── e2e/org_test.go                    # Full gRPC client tests
├── e2e/admin_test.go
└── smoke/smoke_test.go                # Deploy sanity
```

**Structure Decision**: Monorepo with domain-scoped packages under `internal/` matching proto service boundaries (Principle VI, Architecture Constraints). Organizations + Admin are closely related domains sharing `users_orgs` table.

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| [e.g., 4th project] | [current need] | [why 3 projects insufficient] |
| [e.g., Repository pattern] | [specific problem] | [why direct DB access insufficient] |
