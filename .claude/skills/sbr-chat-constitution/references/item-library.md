# sbr-chat-constitution — Candidate Item Library

Progressively-enriched library of candidate constitutional items. Each item
is checked against external presets (via `specify preset search`, or web
search as fallback) before being raised in the interview — if an existing
preset already covers an item well, offer that instead of interviewing from
scratch.

Every run of `sbr-chat-constitution` that resolves an item with no external
preset match is a candidate to be added here, keeping the library growing
from real usage rather than upfront speculation.

Field legend:
- `applies_when` — taxonomy tag(s) that activate this item during triage.
  `any` means it's a candidate for every project regardless of profile.
- `hard_or_soft` — `hard` = non-negotiable once applicable; `soft` =
  default-but-overridable, interview should confirm rather than assume.
- `source_authority` — see taxonomy.md.

---

## architecture

### arch-library-first
- **applies_when**: any
- **hard_or_soft**: soft
- **source_authority**: technical
- **prompt_fragment**: "Every feature starts as a standalone, self-contained,
  independently testable component. No organizational-only components."

### arch-framework-trust
- **applies_when**: any
- **hard_or_soft**: soft
- **source_authority**: technical
- **prompt_fragment**: "Use framework/library features directly rather than
  wrapping them in custom abstractions. Every added layer of abstraction
  requires explicit justification."

### arch-complexity-ceiling
- **applies_when**: `app_type: microservice`, `app_type: monolith`
- **hard_or_soft**: soft
- **source_authority**: technical
- **prompt_fragment**: "Maximum N [services/projects/modules] for initial
  implementation. Additional ones require documented justification."

### arch-service-boundary-grpc
- **applies_when**: `tech_stack: grpc`
- **hard_or_soft**: soft
- **source_authority**: technical
- **prompt_fragment**: "Service boundaries follow [convention]; proto
  definitions are the source of truth for the contract between services."

---

## testing

### test-coverage-threshold
- **applies_when**: any
- **hard_or_soft**: soft
- **source_authority**: technical
- **prompt_fragment**: "Unit test coverage must remain above N%."

### test-realistic-environments
- **applies_when**: any
- **hard_or_soft**: soft
- **source_authority**: technical
- **prompt_fragment**: "Tests must use realistic environments — prefer real
  databases over mocks, actual service instances over stubs. Contract tests
  mandatory before implementation."

### test-first-discipline
- **applies_when**: any
- **hard_or_soft**: soft
- **source_authority**: technical
- **prompt_fragment**: "Tests defining behavior must be written and approved
  before implementation code is generated."

---

## security

### sec-auth-all-endpoints
- **applies_when**: `app_type: microservice`, `client_surface: mobile-native`,
  `client_surface: third-party-api-consumers`
- **hard_or_soft**: hard
- **source_authority**: security
- **prompt_fragment**: "All endpoints must require authentication; no
  unauthenticated access paths without explicit, documented exception."

### sec-secrets-never-committed
- **applies_when**: any
- **hard_or_soft**: hard
- **source_authority**: security
- **prompt_fragment**: "Secrets must never be committed to the repository."

### sec-member-verification-integrity
- **applies_when**: `domain: sharing-economy-community`
- **hard_or_soft**: hard
- **source_authority**: security
- **prompt_fragment**: "Member verification / trust-status logic must not be
  bypassed or weakened for growth, conversion, or convenience reasons."

---

## dependency-governance

### dep-approval-gate
- **applies_when**: any
- **hard_or_soft**: soft
- **source_authority**: technical
- **prompt_fragment**: "Every external dependency must be logged and
  approved before use."

---

## interface-protocol

### iface-versioning-scheme
- **applies_when**: `app_type: microservice`, `app_type: library-sdk`
- **hard_or_soft**: soft
- **source_authority**: technical
- **prompt_fragment**: "API versioning follows [scheme]; breaking changes
  require a new version, not an in-place change to an existing one."

### iface-mobile-backward-compat
- **applies_when**: `client_surface: mobile-native`
- **hard_or_soft**: hard
- **source_authority**: business
- **prompt_fragment**: "Breaking API changes require a deprecation window
  covering at least N app-store release cycles. Old mobile client versions
  cannot be force-updated, so the API must tolerate them for the window."

---

## data-governance

### data-location-privacy
- **applies_when**: `domain: sharing-economy-community`
- **hard_or_soft**: hard
- **source_authority**: security
- **prompt_fragment**: "Member location/proximity data exposed to other
  members is limited to [precision]; retention and access are explicit,
  not implicit defaults."

### data-balance-precision
- **applies_when**: `financial_data_handling: balance-tracking-no-settlement`,
  `financial_data_handling: payment-processing`,
  `financial_data_handling: custody-of-funds`
- **hard_or_soft**: hard
- **source_authority**: technical
- **prompt_fragment**: "All balance/monetary calculations use exact decimal
  arithmetic. Floating-point representation of money values is prohibited."

### data-balance-audit-trail
- **applies_when**: `financial_data_handling: balance-tracking-no-settlement`,
  `financial_data_handling: payment-processing`,
  `financial_data_handling: custody-of-funds`
- **hard_or_soft**: hard
- **source_authority**: business
- **prompt_fragment**: "Every balance-affecting event is recorded in an
  immutable, queryable audit log, sufficient to resolve a member dispute
  against the record."

### data-consumer-privacy-baseline
- **applies_when**: `domain: sharing-economy-community`, `domain: ecommerce-retail`,
  `domain: gaming-media`
- **hard_or_soft**: soft
- **source_authority**: regulatory
- **prompt_fragment**: "Member/user personal data handling follows
  [GDPR/CCPA-style] baseline: purpose limitation, deletion on request,
  minimal retention."

---

## compliance-regulatory

*(No items currently activated for ubertool-backend-trusted — confirmed
2026-08-25 that balance tracking does not trigger regulatory requirements.
Category retained for future projects where it applies, e.g. healthcare/PHI
or payment-processing profiles.)*

---

## business-domain-invariant

### biz-settlement-boundary-guardrail
- **applies_when**: `financial_data_handling: balance-tracking-no-settlement`
- **hard_or_soft**: hard
- **source_authority**: business
- **prompt_fragment**: "The application must never initiate, process, or
  hold money. Balance tracking is informational only; the app does not net
  obligations across more than two parties and does not facilitate
  settlement. This boundary may not be expanded without explicit,
  deliberate governance decision — not as an incidental feature addition."

### biz-large-txn-threshold-boundary
- **applies_when**: `financial_data_handling: balance-tracking-no-settlement`
- **hard_or_soft**: hard
- **source_authority**: business
- **prompt_fragment**: "The threshold above which the app stops tracking a
  balance and defers to member self-settlement is an explicit, centrally
  defined value — not implicit or duplicated across code paths."

### biz-dispute-evidence-support
- **applies_when**: `domain: sharing-economy-community`
- **hard_or_soft**: soft
- **source_authority**: business
- **prompt_fragment**: "The system records condition-at-handoff and
  return-confirmation events sufficient to support resolution of a
  lost/damaged-item dispute between members."

---

## versioning-change-mgmt

### ver-semantic-versioning
- **applies_when**: any
- **hard_or_soft**: soft
- **source_authority**: technical
- **prompt_fragment**: "Constitution and API changes follow semantic
  versioning: MAJOR for incompatible removals/redefinitions, MINOR for new
  additions, PATCH for clarifications."

---

## process-governance

### proc-workflow-conventions
- **applies_when**: any
- **hard_or_soft**: soft
- **source_authority**: business
- **prompt_fragment**: "[e.g. always open a draft PR first; conventional
  commits; required review count before merge]"

---

## scope-expansion-guardrail

### guard-no-unprompted-architecture
- **applies_when**: any
- **hard_or_soft**: hard
- **source_authority**: technical
- **prompt_fragment**: "The agent must never introduce new infrastructure,
  services, or architectural patterns not explicitly called for in the spec
  or plan, without flagging it for human review first. (Derived from
  observed unprompted-scope-expansion failure mode.)"

---

## bugfix

*(Checkpoint gates within the single `sbr-bugfix` skill's workflow — see
si-skills conversation history, 2026-08-25. Kept as one skill with multiple
gates rather than decomposed into separate skills, since ordering is tight
and the linkage-detection plugin has not yet been validated past
single-level delegation. Each item requires a discrete, checkable artifact.

Root-cause analysis branches into three scenarios (added 2026-08-26):
(1) pure programming error, (2) unspecified corner case needing a doc
update, (3) genuine specification gap requiring a new/extended spec.
Scenarios (1)/(2) continue the bugfix path and are confirmed at the
doc-gap-check gate. Scenario (3) is a hard exit to the specification-update
lifecycle — see `bugfix-scenario-3-handoff` — and is not a scope-creep
violation provided the switch is explicit.)*

### bugfix-repro-first
- **applies_when**: any
- **hard_or_soft**: hard
- **source_authority**: technical
- **prompt_fragment**: "No fix code is written before a failing test
  demonstrating the bug exists and is committed/logged."

### bugfix-root-cause-required
- **applies_when**: any
- **hard_or_soft**: hard
- **source_authority**: technical
- **prompt_fragment**: "A recorded root-cause note, distinct from the bug
  report itself, is required before implementing a fix."

### bugfix-root-cause-classification
- **applies_when**: any
- **hard_or_soft**: hard
- **source_authority**: technical
- **prompt_fragment**: "Root-cause analysis must classify the cause into
  exactly one of three scenarios before proceeding: (1) pure programming
  error — a true bug, spec was correct and unambiguous; (2) an unspecified
  corner case — the spec didn't cover this case, and needs a documentation
  update to close the gap; (3) a genuine specification gap — the correct
  behavior requires a new or extended specification, not just a
  documentation clarification. Classification (1) vs (2) is tentative at
  this stage and confirmed at the doc-gap-check gate. Classification (3)
  is a hard branch decided here — see `bugfix-scenario-3-handoff`."

### bugfix-scenario-3-handoff
- **applies_when**: any
- **hard_or_soft**: hard
- **source_authority**: technical
- **prompt_fragment**: "If root-cause classification is scenario (3), the
  workflow must exit the bugfix path and switch to the specification-update
  lifecycle — specification update, design, implementation, testing —
  rather than continuing as a direct code fix. In this repo, that means
  handing off to `sbr-chat-req`/`sbr-chat-sys-design` and the downstream
  `/speckit.plan` → `/speckit.tasks` → `/speckit.implement` pipeline, not
  patching the behavior in place inside `sbr-bugfix`. This branch is the
  required response to a scenario-(3) classification, not an exception
  requiring separate justification — but the switch itself must be
  explicit and visible (a recorded decision, a linked spec-change item),
  not silent."

### bugfix-test-proves-diagnosis
- **applies_when**: any
- **hard_or_soft**: hard
- **source_authority**: technical
- **prompt_fragment**: "The failing test(s) must target the identified root
  cause, not merely reproduce the surface symptom."

### bugfix-green-required
- **applies_when**: any
- **hard_or_soft**: hard
- **source_authority**: technical
- **prompt_fragment**: "A fix is not complete until the previously-failing
  test(s) pass. No merging on an unverified claim of correctness. For
  scenario-(3) classifications (see `bugfix-scenario-3-handoff`), this gate
  is only satisfied once the original reproduction test passes as a result
  of the new/extended specification being implemented through the proper
  spec-update pipeline — not as a shortcut patch applied within the bugfix
  path itself."

### bugfix-doc-gap-check
- **applies_when**: any
- **hard_or_soft**: hard
- **source_authority**: process
- **prompt_fragment**: "Every bugfix includes an explicit, recorded
  documentation-gap assessment — even when the conclusion is 'no gap found.'
  Silence is not compliance. For scenario (1)/(2) classifications, this is
  also where the two are formally confirmed and distinguished: scenario (1)
  concludes 'no doc impact, pure code error'; scenario (2) concludes 'doc
  gap confirmed' and the specification/documentation update is made part
  of this bugfix's deliverable, not deferred."

### bugfix-no-scope-creep
- **applies_when**: any
- **hard_or_soft**: hard
- **source_authority**: technical
- **prompt_fragment**: "A bugfix may not introduce behavior changes or
  refactors beyond what the diagnosed root cause requires. Broader changes
  are flagged as a separate change, not folded in silently. This gate is
  satisfied by definition for scenario (1) and (2) root-cause
  classifications. Scenario (3) is explicitly not a violation of this
  gate — it is the expected outcome of that classification — provided the
  workflow follows `bugfix-scenario-3-handoff` and exits to the
  specification-update pipeline rather than implementing the extended
  behavior silently within `sbr-bugfix` itself."
