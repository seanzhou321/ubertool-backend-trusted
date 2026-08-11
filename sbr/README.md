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

| FR-ID | Requirement Summary | *(one column per test tier, per the adapter's mapping)* | Boundary Status | Planned Tests | Notes |
|---|---|---|---|---|---|

Test-evidence cells cite the concrete unit under test — e.g. `function_name (file:line)` — or
`—` when no plausible match exists. Never a bare "yes"/"covered" with no citation: an
uncited claim of coverage is exactly the false confidence the RTM exists to prevent.

**Planned Tests** is the forward-looking counterpart to the (backward-looking, evidence-only)
tier columns. It is populated only on `Gap`/`Unclassified` rows, and names the specific
not-yet-written test(s) that would close the gap — a proposed function name per tier plus a
one-line description of what it must assert (e.g. `L1: TestFooService_Bar_RejectsX — asserts
the repo write never happens`). This is a proposal for future work, not evidence: it must never
be confused with, or promoted into, a tier-evidence citation until the test actually exists and
has been re-verified by a later audit pass. If the underlying behavior isn't implemented yet
either (the gap is a missing feature, not just a missing test), say so instead of proposing a
test that would have nothing to assert — route it to a feature/bugfix task, not a test stub. If
a requirement's own wording is stale or describes removed/nonexistent functionality (see
`FR-010` in `003-organizations-administration.rtm.md` for a worked example), the right entry
here is "N/A — requirement needs correction, not a test," not a fabricated test name.

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
check-then-write). The SBR paper's own default is that genuinely new development uses gating
mode, reserving audit mode for legacy/retrofit specs. **This project deliberately does not
follow that default** — audit mode (write-after-code) is used for greenfield and
feature-extension work too, not just retrofits. See "Posthoc test-genuineness check" below for
why, and for the substitute discipline that stands in for gating mode's write-first ordering.

### Posthoc test-genuineness check (substitute for pre-implementation gating)

**Why gating mode is relaxed, including for new feature work.** Gating mode's red-then-green
cycle is trustworthy evidence only because the test is written by an author with no
implementation to lean on or be shaped by — that independence is what proves the test
discriminates real behavior from its absence, before the behavior exists to check against.
When test-writing is AI-assisted, that independence isn't reliably available yet: an agent
asked to write a test from a spec, knowing it will also write (or has just written) the
implementation in the same session, has no guaranteed separation between "what the spec says"
and "what I already believe the code does or will do." Until that changes, this project treats
write-after-code as the default for AI-assisted test authoring — both for
`speckit-sbr-bugfix`'s posthoc test-writing and for ordinary test-writing during
`speckit-implement` — rather than claim gating mode's guarantees while the discipline that
earns them (a genuinely blind author) isn't actually in place. This is a deliberate,
revisitable choice: reinstate gating mode for any workflow where blind test authorship is
verified to hold, rather than treating this section as a permanent downgrade.

Losing write-first authorship loses its main benefit — proof, before the fix exists, that the
test *can* fail. A test written after the code it verifies can trivially "pass by
construction," reverse-engineered from what the code already does rather than derived from
what the spec says it should do. That is exactly the "passes accidentally... not evidence"
failure mode this document already warns about (see "Boundary status vocabulary" above). The
check below is the posthoc substitute: instead of proving discriminating power by writing
red-then-green, it proves it by (a) checking the test's assertions against an independently
derived reading of the spec, so implementation details can't leak into what's asserted, and,
where execution is available, (b) breaking the implementation and confirming the test now
fails.

**Two parts, with different operating constraints:**

- **Static spec-fidelity check (Steps A-C)** — pure reading and comparison; no code execution,
  no file modification. Any read-only skill may run this, including `speckit-sbr-audit`.
  - **A. Blind derivation.** Before re-reading the test's own assertions in detail, read only
    the FR/Acceptance-Scenario prose (and the proto/contract definition, if the FR is
    API-shaped) for the behavior under test. From that alone, write down what a correct test
    should assert: the input(s), and the expected observable output, state change, or error —
    nothing else. Do this before studying the test closely, so the derivation isn't anchored to
    what the test already asserts.
  - **B. Compare.** Read the actual test. Check whether its assertions match Step A's
    independently derived expectation, or whether they instead assert incidental
    implementation details the spec never named — an exact SQL string, a private method's call
    count, an internal struct field, a mock verifying its own return value.
  - **C. Classify**, recorded per cited piece of evidence:
    - **Spec-Aligned** — assertions match the independent derivation and are observable at the
      behavior's own boundary (would hold under any correct reimplementation, not just this
      one).
    - **Implementation-Coupled** — assertions describe internals the spec doesn't require.
      Cannot count as `Complete` evidence even though a plausibly-named test exists; route to
      the tier's `Gap` with a note naming what the test actually checks versus what the FR
      requires. If the "internal" detail turns out to be a real hidden requirement, that's a
      candidate for `speckit-sbr-bugfix`'s "missing requirements" root-cause category — not a
      reason to accept the test as-is.
    - **Vacuous** — the test doesn't actually assert the behavior in question (weak or missing
      assertion, swallowed error, assertion on the wrong field). Cannot count as evidence.
- **Dynamic discriminative check (Step D)** — requires running the test suite and a temporary,
  reverted code change; reserved for skills that already execute tests under the adapter's run
  commands (`speckit-sbr-bugfix`). Never `speckit-sbr-audit`, which stays strictly read-only
  per its own operating constraints.
  - **D. Mutate and confirm red.** Temporarily break the specific behavior the test claims to
    assert — the minimal reversible change (invert a condition, comment out a guard, change a
    returned value) — run the test via the adapter's run command, and confirm it now fails for
    that reason. Then revert and confirm green again. A test that stays green through this step
    is vacuous regardless of what Step C concluded from reading it, and must be rewritten
    before it can close a `Gap`.

A test may only back a `Complete` boundary status once it has cleared Step C as **Spec-Aligned**.
`speckit-sbr-bugfix`, which can execute code, additionally requires Step D before accepting a
newly-written test as the evidence that flips a row from `Gap`/`Unclassified` to `Complete`.
`speckit-sbr-audit` runs Steps A-C only, against whatever tests already exist — it cannot run
Step D, so record which check level a row's evidence actually cleared in the RTM's Notes column
(e.g. "genuineness: static only" vs. "genuineness: static + mutation-confirmed") rather than
treating every `Complete` row as equally certain.

### SBR-Trace test annotations (persisted coverage claims)

The static genuineness check's Step A (blind derivation) computes, every time it runs, what a
test *should* assert from the spec alone. That judgment is currently thrown away after each
audit or bugfix run and re-derived from scratch next time. An **SBR-Trace annotation** persists
it: a short doc comment directly above the test function, in the same file, naming the
`FR-XXX`/`AV-ID`/`BUGFIX-NNN` it traces to and the specific behavior or branch it locks down —
not full statement/line coverage (the language's own coverage tooling already reports that for
free), but the semantic claim of what would have to be true about the code for this test to be
meaningful evidence.

- **Placement**: directly above the test function, same file — never a separate manifest. See
  "`sbr/tests/` is intentionally unused" below for why a parallel file was already rejected for
  this exact reason (two files that can drift); an in-file comment moves with the test and shows
  up in the same diff when the test changes, so it doesn't have that failure mode.
- **Exact comment syntax is adapter-specific** (varies by language) — see the adapter's "Search
  conventions for tracing FR-IDs to tests" for this project's concrete Go form and copy-paste
  templates.
- **Adoption is incremental, not a retrofit, but the requirement is forward-universal.**
  `speckit-sbr-bugfix` and `speckit-sbr-feature-upgrade` enforce it themselves at write time.
  Ordinary `speckit-implement` work is bound to the same requirement through Constitution
  Principle IV — both `speckit-implement` and `speckit-tasks` load the constitution as governance
  context on every run — reinforced concretely by a project override at
  `.specify/templates/overrides/tasks-template.md` that bakes the reminder onto every generated
  test task. The pre-existing suite is the one thing this does not reach backward onto:
  `speckit-sbr-audit` is the backfill mechanism there, noting per cited evidence whether an
  annotation exists without downgrading an otherwise-`Complete` row on that basis alone (that
  would fail the entire pre-existing suite against a bar it was never asked to meet at write
  time). Coverage is expected to compound over time, the same way RTM coverage itself did.
- **Drift handling.** An annotation is a claim, and claims go stale. Whenever a test carrying one
  is touched by the static check (Steps A-C), compare the annotation's claim against Step A's
  independently derived expectation and Step B's reading of the actual assertions — a mismatch is
  evidence the test (or the annotation) needs correcting, and is classified exactly like any other
  Implementation-Coupled/Vacuous finding. Never trust the comment over what the test actually
  does.
- **Mechanical presence check**: `make check-sbr-trace` (`sbr/scripts/check-sbr-trace.ps1`) scans
  the adapter's test tiers and reports which `func TestXxx(t *testing.T)` declarations lack an
  `// SBR-Trace:` comment directly above them — a fast, deterministic *presence* check, run
  before the more expensive semantic reasoning in `speckit-sbr-audit` Step 4/5. It is
  **advisory only and always exits 0** — it never fails a build or blocks a commit, matching
  Principle IV's forward-only, non-retroactive scope; it also cannot judge whether an existing
  annotation is *accurate* (drift), only whether one is present — that judgment stays with the
  static genuineness check. It only sees top-level test functions, not `t.Run("...")` subtests,
  so e2e results from it are a weaker signal than unit/integration (see "e2e tests are a trap"
  below).

### Mutation blast radius (extending Step D)

Step D as originally scoped mutates one behavior and checks that the *one* test under validation
goes red. That undersells what a mutation can reveal. Any other test whose SBR-Trace annotation
claims the same `FR-XXX`/`AV-ID`, or that simply lives in the same file/tier and plausibly
exercises the same code path, is also a candidate to flip — and whether it does or doesn't is
informative either way.

When performing Step D: run the tier's sibling tests in the same file alongside the target test
(and, where annotations exist, every test annotated with the same ID across tiers), not just the
target alone. Interpret the results:

- A sibling **annotated as covering this behavior** that **stays green** under the mutation is a
  newly discovered vacuous test — a finding the mutation surfaced beyond what this run was
  originally scoped to fix. Report it explicitly as a follow-up candidate, using the same
  discipline as the hidden-problem review: fold it in only if it's the trivial-identical-fix
  case, otherwise flag it and move on rather than expanding this run's scope.
- A sibling with **no claimed coverage** of this behavior that **flips anyway** is a coupling
  signal — it's sensitive to something more specific than its documented scope, or its
  annotation is incomplete. Note it; this alone is not a defect.
- A sibling that flips and *is* annotated as covering this behavior is the expected case — no
  finding, just confirmation the annotation is accurate.

This is more informative the more annotation coverage already exists, but it's still worth doing
on an unannotated file — "sibling tests in the same file" is cheap to run regardless, and any
surprise is still worth reporting even without a pre-existing claim to compare it against.

### Feature extension vs. feature upgrade vs. bugfix

Three different kinds of change touch existing code in this project, and each gets a different
process because each has a different trigger and a different reliable red-test baseline:

- **Feature extension** — brand new behavior: new classes/packages/RPCs, nothing to contrast
  against. Treated as greenfield. Handled by ordinary `speckit-implement`, with test-writing
  posthoc per "Audit mode vs. gating mode" above and the genuineness check as the substitute for
  gating. There is no prior implementation to be red against, which is exactly why gating mode
  isn't trustworthy here without a genuinely blind author.
- **Feature upgrade** — a *deliberate* change to already-intended behavior: the requirement
  itself is changing, not just its implementation. Handled by `speckit-sbr-feature-upgrade`.
  Unlike extension, there **is** a reliable baseline here — the current implementation is known,
  working, previous-requirement behavior — so a test asserting the *new* intended behavior can be
  run against the *old* code and trusted to go red for a real reason. This project therefore
  keeps true pre-implementation gating for upgrades: spec/design/test updates happen first, and
  the updated tests must fail before implementation starts. See that skill's Goal for its exact
  steps.
- **Bugfix** — an *undiagnosed* symptom or coverage gap: nobody decided to change the
  requirement, something (or nothing, in the pure-gap case) is already wrong relative to intent
  that was never properly verified. Handled by `speckit-sbr-bugfix`, which needs a diagnostic
  root-cause step the other two don't, because unlike a feature upgrade, what's actually broken
  isn't known going in.

The dividing question between upgrade and bugfix is **"did anyone decide this should change, or
did we just discover it's wrong?"** — a deliberate decision is an upgrade; a discovery is a
bugfix, even if the fix ultimately also touches `spec.md` (see "Bug-fix traceability" below for
when a bugfix is allowed to do that).

### Bug-fix step order: reproduce before diagnose

The Appendix B paper's literal cycle runs root-cause analysis (its Step 1) before any test is
written (its Step 2). `speckit-sbr-bugfix` deliberately inverts that ordering: it writes and runs
a reproduction test *first*, then performs root-cause analysis informed by that test's concrete
pass/fail result, then writes any additional root-cause-isolating test the diagnosis calls for.

The reasons: a reproduction test forces the exact symptom to be pinned down precisely before any
theorizing about cause starts, and it gives root-cause analysis a concrete, falsifiable artifact
to reason from — an actual failing assertion and stack trace — rather than code-reading alone,
which is more exposed to "this looks fine to me" false negatives. It also means that when the
target turns out to hide no real bug (Step 2 passes immediately), that outcome is established
before any diagnostic effort is spent, rather than after. The tradeoff accepted: a small amount of
reproduction-test-writing effort is spent even on targets that later turn out not to be bugs — the
project accepts that cost in exchange for grounding both the diagnosis and the eventual
genuineness check in something observed rather than merely read.

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
- **SBR-Trace annotation format for this repo** (see the Portable section's "SBR-Trace test
  annotations" above for the full convention): a `//`-comment block directly above the test
  function, first line `// SBR-Trace: <FR-XXX|AV-ID|BUGFIX-NNN> — <one-line behavior claim>`,
  wrapped to additional `//` lines if the claim needs more than one line. Example:

  ```go
  // SBR-Trace: FR-004 — asserts JoinOrganizationWithInvite dispatches DispatchSilent/email/push
  // to every active ADMIN/SUPER_ADMIN member exactly once on a successful join; does not assert
  // notification content or delivery ordering.
  func TestOrganizationService_JoinOrganizationWithInvite_NotifiesOrgAdmins(t *testing.T) {
  ```

  As of this writing, most existing tests in this repo predate this convention and do not carry
  one yet (`grep -r "SBR-Trace:" tests/` returns few results) — traces in existing RTM rows were
  established by matching requirement prose to test/function intent, not by following this
  convention. Coverage grows incrementally per "Adoption is incremental, not a retrofit" above;
  `speckit-sbr-audit` never edits test files to add annotations itself (it stays read-only), so
  a bare function-name/subtest-string search, as described above, remains necessary for any test
  that doesn't have one yet. Run `make check-sbr-trace` for an instant, mechanical list of which
  test functions currently lack one — advisory only, never fails the build (see the Portable
  section above for what it does and doesn't check).

- **Canonical templates, one per ID namespace** — copy the block, fill the bracketed parts, keep
  it directly above the `func` line with no blank line in between:

  ```go
  // SBR-Trace: FR-XXX — <input(s)> -> <expected observable output/state-change/error>; does not
  // assert <explicitly out-of-scope detail, if any>.
  func TestXxx(t *testing.T) {
  ```

  ```go
  // SBR-Trace: AV-XXX — <attack attempted> is rejected/allowed as <expected outcome> for
  // <endpoint/security-level>; see sbr/rtm/009-security.rtm.md for the finding this closes.
  func TestXxx_Security(t *testing.T) {
  ```

  ```go
  // SBR-Trace: BUGFIX-NNN — regression lock for <the defect>: <input/precondition> now produces
  // <correct behavior> instead of <the old, wrong behavior>.
  func TestXxx_RegressionName(t *testing.T) {
  ```

  The FR/AV forms describe forward-looking intended behavior (what the requirement/attack-vector
  says should happen); the BUGFIX form is the one place the annotation is allowed to also name the
  old, wrong behavior, since the whole point of a regression test is remembering what used to
  break.

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

- Principle I (Reconcile Discrepancies Among Spec, RTM, and Code) — the discrepancy-diagnosis
  discipline `speckit-sbr-bugfix` and `speckit-sbr-feature-upgrade` implement (see "Feature
  extension vs. feature upgrade vs. bugfix" above); note that audit mode being the default here
  is a separate, practical fact (code predates the specs in this retrofit project), not a
  normative claim that code outranks spec/RTM when they disagree.
- Principle IV (Layered Testing Discipline) — the four tiers this adapter maps onto; new
  audit tooling must not invent a fifth.
- Principle VI (Proto-First API Contract) — why `tests/e2e` stands in for L3 contract tests.
