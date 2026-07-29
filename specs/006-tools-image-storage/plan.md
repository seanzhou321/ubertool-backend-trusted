# Implementation Plan: Tools & Image Storage (As-Built + Multi-Org FR-008/FR-009)

**Branch**: `007-tools-image-storage` | **Date**: 2026-07-27 | **Spec**: `specs/006-tools-image-storage/spec.md`

**Input**: Feature specification from `specs/006-tools-image-storage/spec.md`

**Note**: This template is filled in by the `/speckit-plan` command; its definition describes the execution workflow.

## Summary

[Extract from feature spec: primary requirement + technical approach from research]

## Technical Context

**Language/Version**: Go 1.22+

**Primary Dependencies**:
- google.golang.org/grpc
- google.golang.org/protobuf
- github.com/jackc/pgx/v5
- github.com/aws/aws-sdk-go-v2 (S3 presigned URLs)

**Storage**: PostgreSQL + S3-compatible (MinIO local, AWS S3 prod)

**Testing**: Go `testing` + `testify`; tiers per Constitution Principle IV:
- `tests/unit` — pure logic, mocks
- `tests/integration` — service + repo + real DB
- `tests/e2e` — full gRPC client
- `tests/smoke` — deploy sanity

**Target Platform**: Linux server (Podman/EC2)

**Project Type**: gRPC microservice (monorepo)

**Performance Goals**:
- P95 < 200ms for SearchTools, AddTool
- P95 < 500ms for image upload confirmation

**Constraints**:
- Deploy to both Podman + EC2 (Principle V)
- Proto-first (Principle VI)
- Layering: handler → service → repo → DB
- Domain constants for tool status (Principle II)
- Push notifications for mutations (Principle III)

**Scale/Scope**:
- 7 RPCs (Tool) + 6 RPCs (Image Storage)
- Tables: `tools`, `tool_images`
- **Multi-Org Gaps**: FR-008 (cross-org search), FR-009 (shared-org owner info)

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Check | Result |
|-----------|-------|--------|
| I. Code Is Truth | Plan derived from `internal/service/tool.go`, `image_storage.go`, `tool_service.proto`, schema | ✅ PASS |
| II. Domain Model Constraint | `ToolStatus` constants in `internal/domain/tool.go`; no raw strings | ✅ PASS |
| III. Push Notification Pattern | AddTool, UpdateTool, DeleteTool, ConfirmUpload → notifications + email + push | ✅ PASS |
| IV. Layered Testing Discipline | Tests in existing tier directories | ✅ PASS |
| V. Deployment Parity | Schema + S3 config work on Podman + EC2 | ✅ PASS |
| VI. Proto-First API Contract | FR-008 uses the existing `metro`/`organization_id` fields on SearchToolsRequest plus a per-tool shared-org filter — no proto change was needed (`include_all_my_orgs` was never added; see `docs/design/multi-org.md`) | ✅ PASS |

**Gate Status**: ✅ All checks pass — proceed to Phase 0

## Project Structure

### Documentation (this feature)

```text
specs/006-tools-image-storage/
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
│   ├── tool.go                    # ToolService handlers (7 RPCs)
│   └── image_storage.go           # ImageStorageService handlers (6 RPCs)
├── service/
│   ├── tool.go                    # ToolService business logic
│   └── image_storage.go           # ImageStorageService business logic
├── repository/postgres/
│   ├── tool.go                    # ToolRepository
│   └── image_storage.go           # ImageStorageRepository
├── domain/
│   ├── tool.go                    # ToolStatus, Tool, ToolImage
│   └── image_storage.go           # Image types
config/
api/proto/ubertool_trusted_backend/v1/
├── tool_service.proto
└── image_storage_service.proto
podman/trusted-group/postgres/ubertool_schema_trusted.sql

cmd/
├── server/main.go
└── cronjob/main.go                # N/A for Tools/Image Storage

### Test Structure (existing tiers)

tests/
├── unit/tool_service_test.go
├── unit/image_storage_service_test.go
├── unit/pricing_test.go           # Separate pricing utils tests
├── integration/tool_test.go
├── integration/image_storage_test.go
├── e2e/tool_test.go
├── e2e/image_storage_test.go
└── smoke/smoke_test.go
```

**Structure Decision**: Monorepo with domain-scoped packages under `internal/`. Tools + Image Storage are coupled domains (tools own images).

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| [e.g., 4th project] | [current need] | [why 3 projects insufficient] |
| [e.g., Repository pattern] | [specific problem] | [why direct DB access insufficient] |
