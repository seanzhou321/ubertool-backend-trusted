# sbr-chat-constitution — Taxonomy

Single source of truth for the fixed enums used to classify constitutional items
(`references/item-library.md`) and to triage a project during the interview
(`SKILL.md`, Phase A).

Enums are **fixed but extensible**: they do not grow automatically. When the
interview or the item library encounters something that doesn't cleanly match
an existing value in any dimension below, the skill drafts a candidate new
value with a one-line rationale, confirms it with the user before adding it,
and records the addition in that dimension's changelog. Nothing is added
silently or autonomously.

---

## domain

The business/regulatory context the product operates in — what it does for
its end users. Not the same axis as who/what calls the API (see
`client_surface`) or what the tech stack is (see `tech_stack`).

- `fintech`
- `healthcare`
- `gov-public-sector`
- `ecommerce-retail`
- `internal-tooling`
- `gaming-media`
- `developer-infra-platform`
- `sharing-economy-community`
- `general` — default when domain is unclear or doesn't warrant its own value

### Changelog
- `sharing-economy-community` — added for `ubertool-backend-trusted`
  (2026-08-25). Rationale: peer-to-peer trusted-member lending/sharing of
  physical items has no monetary transaction backbone and doesn't fit
  `ecommerce-retail` (which implies a merchant selling to customers).

---

## app_type

The shape of the application itself.

- `microservice`
- `monolith`
- `cli-tool`
- `library-sdk`
- `data-pipeline`
- `mobile-app`
- `web-frontend`
- `batch-job`

### Changelog
(none yet)

---

## tech_stack

Multi-select tags — not mutually exclusive. Describes the technical building
blocks in use.

- `grpc`
- `rest`
- `graphql`
- `go`
- `sql-relational`
- `nosql`
- `event-driven`
- `kubernetes`
- `serverless`
- `data-pipeline`

### Changelog
- `data-pipeline` — added per explicit user request (2026-08-25).

---

## trigger_mode

How the application (or a specific workflow within it) is invoked. Distinct
from `app_type` — a `microservice` can have a `cron-scheduled` sub-workflow
without being reclassified as a `batch-job`.

- `request-driven`
- `cron-scheduled`
- `event-driven-trigger`
- `manual-invocation`

### Changelog
- Dimension added (2026-08-25) to avoid collapsing "cron job" into
  `app_type: batch-job`, which would lose the distinction between a
  scheduling mechanism and an application shape.

---

## client_surface

Who/what calls the API. Separate from `domain` — describes the consumer of
the system, not what the system is for.

- `mobile-native`
- `web-frontend`
- `third-party-api-consumers`
- `internal-service-to-service`
- `iot-device`

### Changelog
- Dimension added (2026-08-25) when "serves mobile apps" was found to answer
  a different question than `domain`.

---

## financial_data_handling

Whether and how the application handles money or money-adjacent data. Kept
separate from `domain` so a non-fintech product can still surface the right
constitutional items when it tracks balances without processing payments.

- `none`
- `balance-tracking-no-settlement`
- `payment-processing`
- `custody-of-funds`

### Changelog
- Dimension added (2026-08-25) for `ubertool-backend-trusted`, which tracks
  member balances and generates monthly bills but never moves money — an
  intermediate case between `none` and `payment-processing` that the
  original taxonomy had no slot for.

---

## source_authority

Who/what makes a given constitutional item non-negotiable. Used to indicate
who can grant an exception or override, if any.

- `business`
- `security`
- `regulatory`
- `technical`

### Changelog
(none yet — expected to extend rarely; this dimension is structural)

---

## category

What kind of constraint the item represents.

- `architecture`
- `testing`
- `security`
- `dependency-governance`
- `interface-protocol`
- `data-governance`
- `compliance-regulatory`
- `performance-sla`
- `business-domain-invariant`
- `versioning-change-mgmt`
- `process-governance`
- `scope-expansion-guardrail`
- `bugfix`

### Changelog
- `bugfix` — added per explicit user request (2026-08-25), to hold
  checkpoint-gate items corresponding to the `sbr-bugfix` skill's internal
  workflow (repro → root cause → diagnostic test → fix → verify → doc-gap
  check), rather than a generic/speculative bugfix category.

---

## ubertool-backend-trusted — reference triage profile

Recorded here as a worked example / regression check for the skill's own
triage logic, not as a taxonomy dimension.

| Dimension | Value |
|---|---|
| `domain` | `sharing-economy-community` |
| `app_type` | `microservice` |
| `tech_stack` | `grpc`, `go`, `sql-relational` *(confirm DB choice)* |
| `trigger_mode` | `request-driven` *(confirm — may also have `cron-scheduled` sub-workflows, e.g. monthly billing)* |
| `client_surface` | `mobile-native` (iOS + Android) |
| `financial_data_handling` | `balance-tracking-no-settlement` |
