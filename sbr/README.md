# SBR for spec-kit

This folder adapts the **Stratified Behavioral Refinement (SBR)** framework — specifically
its trust-boundary and Requirements Traceability Matrix (RTM) concepts — onto this project's
spec-kit setup. It exists in two layers, clearly separated so the portable layer can be lifted
into a reusable spec-kit extension for other projects without dragging this repo's specifics
along with it.

> When porting this to another project: copy **"Portable: SBR-for-spec-kit concepts"**
> unchanged, then rewrite **"This repo's adapter"** for the new project's language, test
> layout, and constitution. Nothing in the portable section should ever need to know this is a
> Go monolith; nothing in the adapter section should ever redefine a portable concept.

## Portable: SBR-for-spec-kit concepts

These ideas apply to any spec-kit project and should not be edited when adapting SBR to a new
repo — only the adapter section below should change.

### What the RTM tracks

The RTM answers one question per functional requirement: **has this requirement's behavior
been formally verified, and where?** It maps each `FR-XXX` in a feature's `spec.md` to the
test evidence that verifies it, across the levels of the project's test pyramid, plus a
grounding/smoke layer. A requirement with no mapped test at any level is a gap — not a
documentation problem, a verification problem.

### Boundary status vocabulary

Every FR-row in an RTM is classified as exactly one of:

- **`Complete`** — the requirement has test evidence at the tier(s) appropriate to its nature
  (not every requirement needs every tier — see below). The Notes column records why the
  covered tiers are sufficient for this particular requirement.
- **`Gap — <tier>`** — no plausible test evidence was found at a tier the requirement should
  reasonably have. Gaps are always named explicitly (which tier, which FR) — never buried in
  an otherwise-green table.
- **`Unclassified`** — the auditor could not confidently determine whether evidence exists
  (e.g. ambiguous naming, requirement too broad to search for). This is flagged for human
  review rather than guessed at. An `Unclassified` row is not a passing row.

A test that "passes accidentally" — one that would pass even if the requirement's behavior
were absent — is not evidence, regardless of what tier it's filed under. When in doubt, prefer
`Unclassified` over a confident-looking but unverified `Complete`.

### RTM schema

One RTM file per audited feature, named `sbr/rtm/<feature-slug>.rtm.md`, with this table:

| FR-ID | Requirement Summary | *(one column per test tier, per the adapter's mapping)* | Boundary Status | Notes |
|---|---|---|---|---|

Test-evidence cells cite the concrete unit under test — e.g. `function_name (file:line)` — or
`—` when no plausible match exists. Never a bare "yes"/"covered" with no citation: an
uncited claim of coverage is exactly the false confidence the RTM exists to prevent.

Each RTM file's header records: the source spec path, a pointer back to this document's
adapter section for the tier mapping in effect, the generation/update date, and the audit
**mode** (see below). Each file's footer is a summary: total FR count, gap count, and an
explicit list of FR-IDs with gaps.

### Audit mode vs. gating mode

The SBR paper's default mode is **gating**: no implementation begins until every test at
every tier is written and confirmed red. That mode assumes greenfield work.

This project's specs are retrofits onto an already-implemented, already-deployed system
(spec-kit's "as-built" convention). For that case the RTM instead runs in **audit mode**:
requirements and code already exist; the RTM's job is to surface where verification is
missing so the gap can be closed deliberately, rather than to gate new generation. Both modes
use the same schema and vocabulary — only the trigger differs (write-then-check vs.
check-then-write). A project doing genuinely new SBR-style development should default new
features to gating mode and reserve audit mode for legacy/retrofit specs, exactly as it would
reserve the paper's Appendix B bug-fix cycle for defects found after the fact.

### Bug-fix traceability: spec update vs. bug-fix RTM

Appendix B's root-cause categories route to different traceability homes, not just different
test tiers. A fix categorized as **missing requirements** is a claim that `spec.md` itself was
incomplete — if the gap is generalizable (any future implementer would need to know this rule,
not just this one caller), the fix must update the owning `spec.md` (new or amended `FR-XXX`,
plus its Acceptance Scenario) and the corresponding row in that feature's
`sbr/rtm/<feature-slug>.rtm.md`, exactly as a `speckit-sbr-audit` gap-closure would. This is the
one case where a bug-fixing skill is allowed to touch `spec.md` — the audit skills never do, but
a bug fix that reveals an actually-missing requirement has to be recorded where every other
requirement lives, or the next audit pass will silently miss it again.

Not every bug earns a new formal requirement, though. A **corner case** — a narrow edge condition
that doesn't generalize into a rule worth stating up front, or a defect whose root cause is a
pure architecture/implementation gap against an already-correct requirement — has nowhere natural
to live in the FR-based RTM schema. These are tracked in a dedicated **bug-fix RTM**, kept
separate from the per-feature RTMs for the same reason the security RTM is separate: its unit of
record is a defect, not a requirement, and forcing it into the FR schema would either inflate
`spec.md` with rules nobody would write from scratch, or leave the fix with no traceability
record at all. See the adapter's "Bug-fix RTM" entry for this project's concrete file and schema.

### Why this matters in practice, not just in theory

Two real gaps were found in this project's own admin authorization logic while writing
as-built specs against existing code, before any RTM tooling existed (see git history:
`da3d301`, `4d32757`). That is the audit-mode value proposition happening by accident, once.
The RTM exists to make that discovery systematic instead of lucky.

### Adversarial (security) audit mode

The audit mode described above is FR-driven: it assumes each requirement is correctly
implemented and only asks "is there a test?" A security audit cannot make that assumption —
the first question is "does a defense exist at all?" — so it runs against **endpoints**, not
`FR-XXX` IDs, and is scoped by the API surface (every RPC/route in the project's proto/route
definitions), not by any single feature's `spec.md`. It still produces an RTM and still uses
`sbr/README.md`'s tier mapping, but with two additions:

- **Unit of audit is the endpoint**, evaluated against a fixed, portable checklist of attack
  categories (OWASP API Security Top 10, or an equivalent industry-standard list named in the
  adapter) rather than requirement prose. The same endpoint is checked for every category that
  plausibly applies to what it does — not every category applies to every endpoint (e.g. a
  static-lookup endpoint with no resource ID has no BOLA surface).
- **Extended boundary-status vocabulary**, layered on top of `Complete`/`Gap`/`Unclassified`:
  - **`Vulnerable`** — the attack **succeeds against current code**. This is a live defect, not
    a test gap: the audit must never silently fold this into `Gap` (which implies the defense
    exists and only the test is missing). Tag with a severity (Critical/High/Medium/Low) and
    cite the exact missing check, ideally by contrasting with a sibling endpoint that gets it
    right (the strongest evidence a gap is a bug, not a design choice).
  - **`Accepted Risk`** — the "attack" succeeds, but an existing test already locks the
    behavior in as intended (its name or an adjacent code comment says so). Listed for a human
    to re-confirm the tradeoff still holds, not as a new discovery.
  - A finding must never be marked `Vulnerable` on pattern-matching alone (e.g. "no `WHERE
    user_id = $1` clause visible in a grep") — read the actual code path handling that request
    before asserting the defense is absent, the same discipline `sbr/README.md`'s "never
    fabricate a trace" rule already requires of FR-based audits.

Because the audit is endpoint-scoped rather than feature-scoped, its RTM does not follow the
"one file per feature" convention below — see the adapter's "Security audit inputs" for where
this project's single, unified security RTM lives and what it covers.

## This repo's adapter

Rewrite this section, and only this section, when porting SBR to a different spec-kit
project.

### Pyramid → test-tier mapping

This is a single Go monolith (`internal/api/grpc` → `internal/service` →
`internal/repository/postgres` → one Postgres schema), not a microservice system. The SBR
paper's L1/L2/L3 assume service-to-service boundaries this project doesn't have, so they are
reused against the **existing, constitutionally-mandated** test tiers (Constitution Principle
IV) rather than introducing a new test category:

| SBR level | This repo's tier | Directory | Run command | What it verifies here |
|---|---|---|---|---|
| L1 — Unit | Unit | `tests/unit` | `make test-unit` | Method/service-function contracts |
| L2 — Component | Integration | `tests/integration` | `make test-integration` (needs a running local Postgres — `make db-deploy` first) | Service + repository logic against a real Postgres instance (this repo has no mocked-DB "component" tier distinct from integration — real-DB integration tests are the closest analog to "service assembly is correct") |
| L3 — Contract | E2E | `tests/e2e` | `make test-e2e` (needs a running server — `make run-precommit` first) | The gRPC/proto surface — `api/proto/.../v1/*.proto` **is** the external contract here (Constitution Principle VI), so full-surface e2e tests stand in for consumer-driven contract tests |
| Grounding | Smoke + E2E journeys | `tests/smoke`, `tests/e2e` | `make test-smoke-ec2` (smoke, against live EC2); e2e journeys run via `make test-e2e` above | System is alive and correctly wired after deployment; e2e journeys are the closest existing analog to full-user-journey BDD scenarios |

`make test-precommit` runs unit + integration + e2e together (still requires Postgres and the
server up first). A tier's run command is the authoritative way to confirm red/green during a
bug fix (see the bug-fix adapter note below) — never infer pass/fail from reading test code.

No new BDD/Gherkin tooling is introduced. The `Given/When/Then` **Acceptance Scenarios**
already written into each `spec.md`'s User Stories serve as this project's BDD specification
layer — they are prose, not executable, but they are the human-readable behavioral contract
the e2e/smoke Go tests are expected to realize.

### Search conventions for tracing FR-IDs to tests

- Go test functions in this repo mostly follow `Test<ServiceName>_<Behavior>` naming — search
  function names first.
- **e2e tests are a trap**: most e2e coverage lives inside `t.Run("...")` subtests under one
  broad `Test<Service>_E2E` function per domain. A top-level function-name grep alone
  under-reports L3 coverage; subtest string literals must be searched too. Cite subtest
  evidence as `TestX_E2E > "subtest name" (file:line)`.
- As of this writing, **no test in this repo cites an FR-ID by number** (`grep -r "FR-" tests/`
  returns nothing) — traces in existing RTMs were established by matching requirement prose
  to test/function intent, not by following an existing convention. Adding `// SBR-Trace:
  FR-XXX` doc comments above traced test functions, going forward, would make future audits
  deterministic instead of inferential — noted as a candidate follow-up, not done
  automatically by the audit skill (see `.claude/skills/speckit-sbr-audit/SKILL.md` — it never
  edits test files).

### `sbr/tests/` is intentionally unused

An earlier draft of this layout considered a manifest folder (`sbr/tests/l1-unit/`, etc.)
mapping FR-IDs to test functions per level, kept separate from the RTM. That was dropped:
tier evidence lives directly in the RTM's columns (see schema above) so there is a single
source of truth instead of two files that can drift. `sbr/scratchpad/` remains available for
temporary working files during analysis.

### Security audit inputs

Inputs the adversarial audit mode (see Portable section above) needs to locate this project's
API surface and its cross-cutting security machinery, so the audit skill never has to
re-derive them by ad hoc searching on every run:

| What | Where |
|---|---|
| Endpoint inventory | `api/proto/ubertool_trusted_backend/v1/*.proto` (one `rpc` per gRPC endpoint) plus any route registered outside the gRPC server — currently only `internal/api/http/image_upload_handler.go`'s `RegisterMockStorageRoutes` |
| Per-endpoint auth requirement | `internal/config/security_config.go` (`EndpointSecurityConfig` map — Public/2FA/Refresh/Access; unmapped methods fail closed to Access, see `GetSecurityLevel`) |
| Auth enforcement point | `internal/api/grpc/interceptor/auth_interceptor.go` (JWT validation, security-level gate, injects `user-id` into gRPC metadata as a hard overwrite) |
| Token issuance/validation | `internal/security/token.go` (HS256 JWT, `TokenManager`) |
| Rate limiting | `internal/security/rate_limiter.go` + `internal/api/grpc/interceptor/rate_limit_interceptor.go` (as of this writing, covers only `AuthService/Login` and `/Verify2FA` — every other endpoint is unthrottled; the audit must re-verify this scope each run rather than assume it) |
| Attack-category checklist | OWASP API Security Top 10 (2023): API1 Broken Object Level Authorization, API2 Broken Authentication, API3 Broken Object Property Level Authorization, API4 Unrestricted Resource Consumption, API5 Broken Function Level Authorization, API6 Unrestricted Access to Sensitive Business Flows, API7 Server Side Request Forgery, API8 Security Misconfiguration, API9 Improper Inventory Management, API10 Unsafe Consumption of APIs |
| Output RTM | `sbr/rtm/009-security.rtm.md` — a single unified file covering all services (unlike the per-feature `001`-`008` RTMs), organized into one section per gRPC service plus a "Global / Cross-Cutting" section and an HTTP-endpoints section |

### Bug-fix RTM

Corner-case defects fixed via `speckit-sbr-bugfix` (see Portable section above, "Bug-fix
traceability: spec update vs. bug-fix RTM") that don't warrant a `spec.md`/FR change live in
`sbr/rtm/bugfix-*.rtm.md` — currently a single file, `sbr/rtm/bugfix-general.rtm.md`, but the
glob (not a fixed name or the `001`/`009`-style numeric prefix used by feature and security RTMs)
is the actual contract: it lets the file exist under any descriptive name, and lets a project
split into more than one bugfix RTM later (e.g. by domain) without a rename or renumbering.
Appended to as fixes land, never rewritten per feature. Generalizable missing-requirement fixes
instead update the owning feature's `spec.md` and its `sbr/rtm/<feature-slug>.rtm.md` — they
never go in a bugfix RTM.

### Constitution cross-reference

- Principle I (Code Is Truth) — why audit mode, not gating mode, is the default here.
- Principle IV (Layered Testing Discipline) — the four tiers this adapter maps onto; new
  audit tooling must not invent a fifth.
- Principle VI (Proto-First API Contract) — why `tests/e2e` stands in for L3 contract tests.
