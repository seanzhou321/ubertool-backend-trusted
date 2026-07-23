<!--
Sync Impact Report
==================
Version change: (unratified template) → 1.0.0
Rationale: Initial ratification. This project (ubertool-backend-trusted) is a mature,
already-deployed codebase; the constitution is being retrofitted to describe established
practice, not to introduce new aspirational rules. Bump is MAJOR-equivalent (first concrete
adoption) but recorded as 1.0.0 per spec-kit's initial-ratification convention.

Modified principles: n/a (initial fill from placeholder template)
Added sections:
  - Core Principles I-VI (Code Is Truth, Domain Model Constraint Strategy,
    Push Notification Pattern, Layered Testing Discipline, Deployment Parity,
    Proto-First API Contract)
  - Architecture Constraints
  - Development Workflow
  - Governance
Removed sections: none (all placeholder slots filled)

Templates requiring updates:
  - .specify/templates/plan-template.md: ✅ no change needed (Constitution Check gate is
    generic and reads from this file at plan time)
  - .specify/templates/spec-template.md: ✅ no change needed (no principle-specific
    references)
  - .specify/templates/tasks-template.md: ✅ no change needed (test-tier phrasing is
    generic; task authors should map onto tests/unit|integration|e2e|smoke per Principle IV)
  - .claude/skills/speckit-*/SKILL.md: ✅ reviewed, no agent-specific or outdated
    references requiring correction

Follow-up TODOs: none. RATIFICATION_DATE set to the date this constitution was first
completed (today), since no prior ratified version existed.
-->

# ubertool-backend-trusted Constitution

## Core Principles

### I. Code Is Truth

The actual behavior of `internal/service`, `internal/repository`, `internal/api/grpc`, and
the schema in `podman/trusted-group/postgres/ubertool_schema_trusted.sql` is the single
source of truth for how the system behaves. `docs/` — including `docs/design/*` and
especially `docs/improvements/*` — MAY be stale proposals, superseded critiques, or
aspirational notes that were never implemented (or were implemented differently than
written). `docs/design/grpc_api_business_logic.md` is the most-maintained business-logic
reference and SHOULD be treated as the first place to look, but even it MUST be verified
against the code before being relied upon.

Any spec, plan, task, or other document produced under this constitution MUST be derived
by reading the real implementation and schema, not by transcribing an existing doc. Where
a doc and the code disagree, the document being written MUST say so explicitly rather than
silently picking one side.

**Rationale**: This codebase has accumulated documentation across multiple development
phases with no enforced sync mechanism. Treating docs as authoritative has already produced
at least one confirmed false lead (`docs/improvements/bill-split-api-review.md` describes an
`AcknowledgePayment` bug that the current code does not have). Trusting code first prevents
propagating stale claims into new specs.

### II. Domain Model Constraint Strategy

Status-like fields (e.g. bill status) are intentionally kept as `string`/`TEXT` at the proto
and database layers to allow new values to be added without proto-breaking changes or DB
migrations. This flexibility is NOT a license to use raw string literals in application
code: the service layer MUST use typed `domain.*` constants (e.g. `domain.BillStatusPending`)
for every read, write, and comparison of such fields. New status-like fields introduced in
future work MUST follow the same pattern: loose at the proto/DB boundary, strictly typed
inside `internal/service`.

**Rationale**: Established and documented in `docs/design/grpc_api_business_logic.md`;
preserves forward compatibility for new statuses while still catching typos and invalid
values at compile time where it matters most.

### III. Push Notification Pattern

For every event that creates an in-app `notifications` row AND sends an email, a push
notification MUST also be sent to the same recipient(s), with the sole exception of events
during sign-on, user identification, user provisioning, and invitations (2FA codes,
invitation emails, signup flows, join request submissions). The implementation pattern is:
insert the `notifications` row first to obtain a `notification_id`, send the email, then
look up active `fcm_tokens` for the recipient(s) and dispatch push notifications
asynchronously, marking tokens `OBSOLETE` on `messaging.IsUnregistered`. Any new
notify-worthy event added to the system MUST follow this three-channel pattern unless it
falls under the documented exceptions.

**Rationale**: Matches the pattern already implemented consistently across Admin,
Organization, Bill Split, Rental, and Notification service code, and documented in
`docs/design/grpc_api_business_logic.md`.

### IV. Layered Testing Discipline

`tests/unit`, `tests/integration`, `tests/e2e`, and `tests/smoke` are the four established
test tiers for this project. New functional work MUST land its tests in the tier(s)
appropriate to what is being verified (pure logic → unit; service+repo+DB → integration;
full gRPC surface → e2e; deploy sanity → smoke), rather than inventing new test
directories or conventions. spec-kit `tasks.md` output for this project MUST map its test
tasks onto these existing tiers.

**Rationale**: The tiers already exist and are populated (see `tests/` directory);
consistency here keeps generated tasks executable against the real test suite instead of
proposing a parallel structure.

### V. Deployment Parity

This service is deployed to two targets: a local Podman machine on Windows 11
(`podman/trusted-group`) and an AWS EC2 micro instance (`deploy/ec2-mvp`), each running its
own PostgreSQL instance provisioned from the same schema
(`podman/trusted-group/postgres/ubertool_schema_trusted.sql`). Any schema change,
configuration change, or new environment-dependent behavior MUST remain deployable to both
targets — a change that only works in one environment is incomplete.

**Rationale**: Both environments are actively used (`deploy/local_db` and `deploy/ec2-mvp`
both exist and are maintained); drift between them has historically been a source of
deploy-time surprises (see the recent SES/SMTP and config-consolidation work in git
history).

### VI. Proto-First API Contract

`api/proto/ubertool_trusted_backend/v1/*.proto` defines the external API surface;
`api/gen/v1/*` is generated code and MUST NEVER be hand-edited. Any API surface change
starts with a proto change, followed by regeneration, followed by handler/service
implementation. Handlers in `internal/api/grpc` are thin adapters that translate between
proto messages and domain/service calls — business logic MUST live in `internal/service`,
not in the handler layer.

**Rationale**: Matches the existing layering (`internal/api/grpc` → `internal/service` →
`internal/repository/postgres`) and prevents generated-code drift.

## Architecture Constraints

- **Layering**: `internal/api/grpc` (transport/adapter) → `internal/service` (business
  logic) → `internal/repository/postgres` (persistence) → PostgreSQL. Business logic MUST
  NOT leak into the transport or repository layers.
- **HTTP surface**: `internal/api/http` exists alongside gRPC for endpoints that need it
  (e.g. presigned upload flows); it follows the same layering rule.
- **Configuration**: `internal/config` centralizes environment-dependent settings; new
  config MUST support both deployment targets (Principle V), not hardcode one environment's
  values.
- **Scheduled/background work**: `internal/jobs`, `internal/scheduler`, and
  `cmd/cronjob` are the established locations for background/periodic work (e.g. bill
  auto-resolution, dispute escalation timers); new periodic behavior belongs there, not in
  request-handling paths.

## Development Workflow

- New feature work under spec-kit follows spec → plan → tasks → (implement |
  converge), scoped per service domain (Auth, Admin, Organizations, Bill Split, Tools,
  Rentals, Ledger, Notifications, Image Storage, Users) to match the existing proto service
  boundaries in `api/proto/ubertool_trusted_backend/v1`.
- Because this is a retrofit onto an existing system, specs for already-implemented
  functionality MUST be written as accurate "as-built" descriptions (Principle I), and
  `tasks.md` for such domains MUST distinguish genuinely missing/weak work (e.g. absent
  test coverage) from already-complete work — not regenerate work that already exists.
  `/speckit-converge` is the preferred tool for detecting genuine gaps against an as-built
  spec.
- Generated code (`api/gen/v1`) and vendored/third-party code are out of scope for
  constitution compliance review.

## Governance

This constitution supersedes ad hoc practice where the two conflict. Amendments are made by
editing `.specify/memory/constitution.md` directly, following the same Sync Impact Report
convention used here, and require:

1. A version bump per semantic versioning: MAJOR for backward-incompatible principle
   removal/redefinition, MINOR for a new principle or materially expanded guidance, PATCH
   for clarifications/wording.
2. A check of `.specify/templates/plan-template.md`, `spec-template.md`, and
   `tasks-template.md` for now-outdated references.
3. An updated `Last Amended` date below.

All specs, plans, and tasks produced for this project MUST be checked against these
principles before being finalized; a principle violation found during `/speckit-converge`
or manual review is treated as CRITICAL severity and MUST be remediated or explicitly
justified (with simpler alternatives rejected) rather than silently ignored.

**Version**: 1.0.0 | **Ratified**: 2026-07-22 | **Last Amended**: 2026-07-22
