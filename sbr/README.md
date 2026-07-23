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

### Why this matters in practice, not just in theory

Two real gaps were found in this project's own admin authorization logic while writing
as-built specs against existing code, before any RTM tooling existed (see git history:
`da3d301`, `4d32757`). That is the audit-mode value proposition happening by accident, once.
The RTM exists to make that discovery systematic instead of lucky.

## This repo's adapter

Rewrite this section, and only this section, when porting SBR to a different spec-kit
project.

### Pyramid → test-tier mapping

This is a single Go monolith (`internal/api/grpc` → `internal/service` →
`internal/repository/postgres` → one Postgres schema), not a microservice system. The SBR
paper's L1/L2/L3 assume service-to-service boundaries this project doesn't have, so they are
reused against the **existing, constitutionally-mandated** test tiers (Constitution Principle
IV) rather than introducing a new test category:

| SBR level | This repo's tier | Directory | What it verifies here |
|---|---|---|---|
| L1 — Unit | Unit | `tests/unit` | Method/service-function contracts |
| L2 — Component | Integration | `tests/integration` | Service + repository logic against a real Postgres instance (this repo has no mocked-DB "component" tier distinct from integration — real-DB integration tests are the closest analog to "service assembly is correct") |
| L3 — Contract | E2E | `tests/e2e` | The gRPC/proto surface — `api/proto/.../v1/*.proto` **is** the external contract here (Constitution Principle VI), so full-surface e2e tests stand in for consumer-driven contract tests |
| Grounding | Smoke + E2E journeys | `tests/smoke`, `tests/e2e` | System is alive and correctly wired after deployment; e2e journeys are the closest existing analog to full-user-journey BDD scenarios |

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

### Constitution cross-reference

- Principle I (Code Is Truth) — why audit mode, not gating mode, is the default here.
- Principle IV (Layered Testing Discipline) — the four tiers this adapter maps onto; new
  audit tooling must not invent a fifth.
- Principle VI (Proto-First API Contract) — why `tests/e2e` stands in for L3 contract tests.
