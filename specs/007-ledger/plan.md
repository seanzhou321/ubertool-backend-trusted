# Implementation Plan: Ledger (As-Built + Multi-Org Gap)

**Branch**: `008-ledger` | **Date**: 2026-07-27 | **Spec**: `specs/007-ledger/spec.md`

**Input**: Feature specification from `specs/007-ledger/spec.md`

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
- P95 < 200ms for GetBalance, GetTransactions, GetLedgerSummary
- Support 10k+ members per org, 100+ orgs

**Constraints**: 
- Must deploy to both Podman (local) and EC2 (prod) per Constitution Principle V
- Proto-first API (Principle VI): changes start in `api/proto/.../ledger_service.proto`
- Layering: handler → service → repository → DB (Architecture Constraints)
- Domain constants for status-like fields (Principle II)
- Push notification pattern for mutations (Principle III) — N/A for read-only Ledger

**Scale/Scope**: 
- 3 RPCs: GetBalance, GetTransactions, GetLedgerSummary
- Tables: `ledger_transactions`, `users_orgs` (balance_cents per membership)
- **Gap**: FR-004 (cross-org ledger rollup) — Known Discrepancy Gap 2: as-built returns error on `org_id=0`

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Check | Result |
|-----------|-------|--------|
| I. Code Is Truth | Plan derived from `internal/service/ledger.go`, `internal/repository/postgres/ledger.go`, `api/proto/.../ledger_service.proto`, schema SQL | ✅ PASS |
| II. Domain Model Constraint | No status-like fields in Ledger (read-only); balance_cents uses typed domain constants in service layer | ✅ PASS |
| III. Push Notification Pattern | Ledger is read-only — no mutations, no notifications needed | ✅ PASS (N/A) |
| IV. Layered Testing Discipline | Tests map to existing `tests/unit`, `tests/integration`, `tests/e2e`, `tests/smoke` tiers | ✅ PASS |
| V. Deployment Parity | Schema changes work on both Podman and EC2 PostgreSQL instances | ✅ PASS |
| VI. Proto-First API Contract | Any API change starts with proto regeneration | ✅ PASS |

**Gate Status**: ✅ All checks pass — proceed to Phase 0

## Project Structure

### Documentation (this feature)

```text
specs/007-ledger/
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
├── api/grpc/ledger.go           # gRPC handlers (thin adapters)
├── service/ledger.go            # Business logic
├── repository/postgres/ledger.go # Data access
├── domain/ledger.go             # Domain types & constants
config/
api/proto/ubertool_trusted_backend/v1/ledger_service.proto  # Proto contract
podman/trusted-group/postgres/ubertool_schema_trusted.sql   # Schema

cmd/
├── server/main.go               # gRPC server entrypoint
└── cronjob/main.go              # Background jobs (N/A for Ledger)

### Test Structure (existing tiers)

tests/
├── unit/ledger_service_test.go        # Pure logic tests
├── integration/ledger_test.go         # Service + repo + real DB
├── e2e/ledger_test.go                 # Full gRPC client tests
└── smoke/smoke_test.go                # Deploy sanity
```

**Structure Decision**: Monorepo with domain-scoped packages under `internal/` matching proto service boundaries (Principle VI, Architecture Constraints). Ledger is a read-only domain with 3 RPCs.

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| [e.g., 4th project] | [current need] | [why 3 projects insufficient] |
| [e.g., Repository pattern] | [specific problem] | [why direct DB access insufficient] |
