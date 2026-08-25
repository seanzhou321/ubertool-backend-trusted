<!--
Sync Impact Report
==================
Version change: 1.0.0 → 2.0.0 | Last Amended: 2026-08-10
Rationale: This amendment was drafted in several increments in one sitting (1.1.0, 2.0.0, 2.1.0,
2.2.0 were each considered in turn); none of those intermediate versions was ever the current
version any spec, plan, task, or line of code was produced against — nothing had been committed
between them — so they are collapsed into the single release actually being adopted, per the
same reasoning that keeps this file down to one current Sync Impact Report rather than a
preserved copy per revision: a version number should mark an adopted state, not an editing step.
Net changes from 1.0.0:

1. Principle I redefined from "Code Is Truth" to "Reconcile Discrepancies Among Spec, RTM, and
   Code" (MAJOR — this is why the release is 2.0.0, not 1.x). This project started as a
   brownfield codebase with no maintained spec — defaulting to code-as-truth was the right call
   for that phase, and the as-built extraction it drove surfaced real critical security issues in
   the admin-authorization logic (see sbr/README.md → "Why this matters in practice"). Now that
   spec.md and the RTM exist as intentionally maintained artifacts for all eight feature domains,
   keeping that same code-wins default no longer helps and actively biases every investigation
   toward "the code must be right" — the opposite of what the RTM exists to check. Every prior
   "Code Is Truth" citation across specs 001-008, their plans/research, and sbr/README.md was
   updated to the new title in the same pass.
2. Principle IV extended with the SBR-Trace test-annotation requirement: every new/modified test
   MUST carry a persisted coverage claim. Full detail in sbr/README.md -> "SBR-Trace test
   annotations" — this principle states the requirement and its enforcement points, not the
   annotation format.
3. That requirement's enforcement scope covers ordinary `speckit-implement` work, not just the
   SBR skills. `speckit-implement` and `speckit-tasks` already load this file as governance
   context on every run (the same channel Principles II/III already use to bind a vendored,
   unedited command) — the requirement reaches them through that, reinforced concretely by a new
   `.specify/templates/overrides/tasks-template.md` that bakes an SBR-Trace reminder into every
   generated test task, resolved via spec-kit's own project-override-over-shared-core template
   stack (see `.specify/scripts/powershell/common.ps1`'s `Resolve-Template`). The pre-existing
   suite remains excluded (surfaced by `sbr-audit` as a gap, not a violation) — only the
   *forward* scope changed.
4. Development Workflow gained one bullet naming the SBR skills (`sbr-audit`,
   `sbr-bugfix`, `sbr-feature-upgrade`, `sbr-close-gaps`) and pointing to
   `sbr/README.md` as their authoritative detail, in the same style as the existing
   `/speckit-converge` bullet.

Modified principles: I (renamed and redefined), IV (extended and widened)
Added sections: none (one bullet added to existing Development Workflow section)
Removed sections: none
Templates checked: plan-template.md, spec-template.md — no changes needed. tasks-template.md —
project override added at .specify/templates/overrides/tasks-template.md (base template under
.specify/templates/ left untouched, so a future `specify upgrade` doesn't conflict with it).
sbr/README.md and .claude/skills/sbr-{audit,bugfix,feature-upgrade}/SKILL.md already
carry the enforcement detail this amendment points to.
Follow-up TODOs: none.
-->

# ubertool-backend-trusted Constitution

## Core Principles

### I. Reconcile Discrepancies Among Spec, RTM, and Code

This project has completed its initial as-built extraction: `spec.md` exists for every feature
domain, and each has an accompanying `sbr/rtm/<feature-slug>.rtm.md` tracing its requirements to
test evidence. That extraction defaulted to trusting `internal/service`, `internal/repository`,
`internal/api/grpc`, and the schema over documentation whenever they disagreed — this
constitution's original "Code Is Truth" formulation — because at that point there was no
maintained specification to trust instead. That default no longer holds now that `spec.md` and
the RTM are themselves intentionally maintained, authoritative artifacts, not one-time extraction
output.

Going forward, a disagreement among the specification, the RTM, and the code is a **discrepancy
to be diagnosed and resolved**, not a signal to silently defer to whichever one currently matches
production behavior. Any of the three can be the one that's wrong: the spec can be missing or
ambiguous, the RTM can cite a test that doesn't actually verify what it claims to (see
`sbr/README.md` → "Posthoc test-genuineness check"), or the code can simply be buggy. The
mechanism for resolving this is the SBR discipline: `sbr-bugfix`'s root-cause
categorization decides whether a defect traces to a missing/ambiguous requirement (spec is wrong)
or an architecture/implementation gap (code is wrong), and `sbr-feature-upgrade` handles
the case where the requirement is being deliberately changed. No spec-kit document produced under
this constitution may resolve a spec/RTM/code disagreement by silent preference for any one of
the three — it must be diagnosed.

`docs/` outside of `spec.md`/`sbr/` — informal design notes and proposals, especially
`docs/design/*` and `docs/improvements/*` — remains a separate, lower-trust category: these were
never subject to the RTM's verification discipline and MAY be stale, superseded, or aspirational.
`docs/design/grpc_api_business_logic.md` is the most-maintained business-logic reference and
SHOULD be treated as a useful lead, but — like the rest of `docs/` — is not evidence on its own
and any claim drawn from it still needs to be checked against `spec.md`/RTM/code.

**Rationale**: This project started as a brownfield codebase with no maintained specification —
trusting code over documentation was the right default for that phase, and it did real work:
treating stale docs as authoritative had already produced at least one confirmed false lead
(`docs/improvements/bill-split-api-review.md` describes an `AcknowledgePayment` bug the code
does not have), and the as-built extraction this principle drove surfaced genuine critical
security gaps in the admin-authorization logic before any RTM tooling existed (see
`sbr/README.md` → "Why this matters in practice, not just in theory"). But an unqualified
preference for code, kept in force after `spec.md` and the RTM exist as maintained artifacts for
every feature domain, now actively works against the RTM's own purpose: it would bias every
investigation toward "the code must be right, the spec must be wrong" instead of asking which one
actually is. This is a foundational change going forward, not a historical footnote — every
existing citation of the original formulation across this project's specs, plans, and research
docs has been updated to this principle's current title, on the same reasoning that keeps this
file itself down to one current Sync Impact Report: git history is the record of what this
constitution used to say, not a copy preserved in the document itself.

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

Every new or modified test — regardless of which command wrote it — MUST carry an `SBR-Trace`
annotation: a persisted claim of which requirement/finding it verifies and the specific behavior
it locks down, checked against drift by the static genuineness check on every later run that
touches it (see `sbr/README.md` → "SBR-Trace test annotations" for the convention). This applies
to `sbr-bugfix` and `sbr-feature-upgrade` directly — they enforce it themselves,
at write time, as part of their own steps — and to ordinary `speckit-implement` work through this
constitution: `speckit-implement` and `speckit-tasks` both unconditionally load this file as
governance context before acting, the same channel Principles II and III already rely on to bind
a vendored, unedited command to a project-specific rule. `.specify/templates/overrides/tasks-template.md`
reinforces this concretely by adding the requirement directly to every generated test task, so it
reaches `speckit-implement` as a literal item on the plan it already executes task-by-task, not
only as prose it has to remember to apply. This requirement does not reach *backward*: the
pre-existing suite is not retroactively in violation for lacking annotations — `sbr-audit`
surfaces those as a visible gap for incremental pickup, not a CRITICAL finding, and coverage is
expected to compound the same way RTM coverage itself did, not appear complete on day one.

**Rationale**: The tiers already exist and are populated (see `tests/` directory);
consistency here keeps generated tasks executable against the real test suite instead of
proposing a parallel structure. The annotation requirement was originally scoped to exclude
`speckit-implement`-authored tests on the assumption that a vendored command's behavior couldn't
be reached without editing it — that assumption was wrong: `speckit-implement` already reads this
file as governance context on every run (`speckit-implement/SKILL.md`'s own "Load context" step),
which is exactly how Principles II and III already govern its behavior today without either
principle ever touching `speckit-implement`'s own file. The task-template override adds a second,
concrete channel on top of that for this specific requirement.

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
- Bug fixes, RTM gap-closure, and deliberate changes to already-implemented behavior follow the
  SBR (Stratified Behavioral Refinement) discipline in `sbr/README.md`, via `sbr-audit`
  (read-only RTM tracing), `sbr-bugfix` (diagnosed defects), `sbr-feature-upgrade`
  (deliberate requirement changes), and `sbr-close-gaps` (batch gap-closure) —
  `sbr/README.md` is the authoritative detail, not this constitution.

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

**Version**: 2.0.0 | **Ratified**: 2026-07-22 | **Last Amended**: 2026-08-10
