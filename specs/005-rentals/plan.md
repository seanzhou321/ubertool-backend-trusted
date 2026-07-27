# Implementation Plan: Rentals (As-Built + Multi-Org FR-008)

**Branch**: `006-rentals` | **Date**: 2026-07-27 | **Spec**: `specs/005-rentals/spec.md`

**Input**: Feature specification from `specs/005-rentals/spec.md`

**Note**: This template is filled in by the `/speckit-plan` command; its definition describes the execution workflow.

## Summary

[Extract from feature spec: primary requirement + technical approach from research]

## Technical Context

**Language/Version**: Go 1.22+

**Primary Dependencies**:
- google.golang.org/grpc
- google.golang.org/protobuf
- github.com/jackc/pgx/v5
- github.com/golang-migrate/migrate/v4

**Storage**: PostgreSQL (schema: `podman/trusted-group/postgres/ubertool_schema_trusted.sql`)

**Testing**: Go `testing` + `testify`; tiers per Constitution Principle IV:
- `tests/unit` — pure logic, mocks
- `tests/integration` — service + repo + real DB
- `tests/e2e` — full gRPC client
- `tests/smoke` — deploy sanity

**Target Platform**: Linux server (Podman/EC2)

**Project Type**: gRPC microservice (monorepo)

**Performance Goals**:
- P95 < 200ms for CreateRentalRequest, ApproveRentalRequest
- P95 < 500ms for ListMyRentals/ListMyLendings

**Constraints**:
- Deploy to both Podman + EC2 (Principle V)
- Proto-first (Principle VI)
- Layering: handler → service → repo → DB
- Domain constants for rental status (Principle II)
- Push notifications for mutations (Principle III)

**Scale/Scope**:
- 16 RPCs (largest domain)
- Tables: `rentals`, `tools` (via Tools domain)
- **Multi-Org Gap (FR-008)**: Context switch when renting from different org

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Check | Result |
|-----------|-------|--------|
| I. Code Is Truth | Plan derived from `internal/service/rental.go`, `internal/api/grpc/rental.go`, `rental_service.proto`, schema | ✅ PASS |
| II. Domain Model Constraint | `RentalStatus` constants in `internal/domain/rental.go`; no raw strings | ✅ PASS |
| III. Push Notification Pattern | Create/Approve/Finalize/Complete/Cancel all trigger notifications + email + push | ✅ PASS |
| IV. Layered Testing Discipline | Tests in existing tier directories | ✅ PASS |
| V. Deployment Parity | Schema works on Podman + EC2 PostgreSQL | ✅ PASS |
| VI. Proto-First API Contract | FR-008 adds `current_organization_id` to CreateRentalRequest — proto change | ✅ PASS |

**Gate Status**: ✅ All checks pass — proceed to Phase 0

## Project Structure

### Documentation (this feature)

```text
specs/005-rentals/
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
├── api/grpc/rental.go              # RentalService handlers (16 RPCs)
├── service/rental.go               # Business logic (1058 lines)
├── repository/postgres/rental.go   # Data access
├── domain/rental.go                # RentalStatus, Rental, PricingSnapshot
├── utils/pricing.go                # CalculateRentalCost (own test suite)
config/
api/proto/ubertool_trusted_backend/v1/rental_service.proto
podman/trusted-group/postgres/ubertool_schema_trusted.sql

cmd/
├── server/main.go
└── cronjob/main.go                 # N/A for Rentals

### Test Structure (existing tiers)

tests/
├── unit/rental_service_test.go
├── unit/pricing_test.go
├── integration/rental_test.go
├── e2e/rental_test.go
└── smoke/smoke_test.go
```

**Structure Decision**: Monorepo with domain-scoped packages under `internal/`. Rentals is the largest domain (16 RPCs) and depends on Tools + Organizations + Bill Split.

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| [e.g., 4th project] | [current need] | [why 3 projects insufficient] |
| [e.g., Repository pattern] | [specific problem] | [why direct DB access insufficient] |
