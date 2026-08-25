---
name: "sbr-bugfix"
description: "Fix a single bug, RTM gap, or security finding using the SBR framework's Appendix B bug-fix discipline: interview for reproduction detail, reproduce, diagnostic root-cause analysis, failing tests before the fix, verification, hidden-problem review, and a post-fix spec/RTM re-examination."
argument-hint: "<bug description | FR-ID | AV-ID | file:line> — required, the single defect to fix"
compatibility: "Requires spec-kit project structure with sbr/README.md's adapter, including its test-tier run commands"
metadata:
  author: "sbr-extension"
  source: "tdd-bdd-sbr.pdf Appendix B — Bug Fixes in the SBR Framework"
user-invocable: true
disable-model-invocation: false
---


## User Input

```text
$ARGUMENTS
```

You **MUST** consider the user input before proceeding. This skill requires a single, concrete
target — a bug description, an `FR-XXX` ID (resolved against `sbr/rtm/<feature-slug>.rtm.md`), an
`AV-ID` (resolved against the security RTM named in the adapter), or a `file:line` pointer. If
`$ARGUMENTS` is empty, STOP and ask the user what to fix rather than guessing a target — unlike
the audit skills, this one writes code, so it must never invent its own scope.

## Pre-Execution Checks

**Check for extension hooks (before bugfix)**:

- Check if `.specify/extensions.yml` exists in the project root.
- If it exists, read it and look for entries under the `hooks.before_sbr_bugfix` key
- If the YAML cannot be parsed or is invalid, skip hook checking silently and continue normally
- Filter out hooks where `enabled` is explicitly `false`. Treat hooks without an `enabled` field as enabled by default.
- For each remaining hook, do **not** attempt to interpret or evaluate hook `condition` expressions:
  - If the hook has no `condition` field, or it is null/empty, treat the hook as executable
  - If the hook defines a non-empty `condition`, skip the hook and leave condition evaluation to the HookExecutor implementation
- When constructing slash commands from hook command names, replace dots (`.`) with hyphens (`-`). For example, `speckit.git.commit` → `/speckit-git-commit`.
- For each executable hook, output the following based on its `optional` flag:
  - **Optional hook** (`optional: true`):

    ```text
    ## Extension Hooks

    **Optional Pre-Hook**: {extension}
    Command: `/{command}`
    Description: {description}

    Prompt: {prompt}
    To execute: `/{command}`
    ```

  - **Mandatory hook** (`optional: false`):

    ```text
    ## Extension Hooks

    **Automatic Pre-Hook**: {extension}
    Executing: `/{command}`
    EXECUTE_COMMAND: {command}

    Wait for the result of the hook command before proceeding to the Goal.
    ```
    After emitting the block above you MUST actually invoke the hook and wait for it to finish before continuing.

- If no hooks are registered or `.specify/extensions.yml` does not exist, skip silently

## Goal

Fix exactly one defect — a reported bug, an RTM `Gap`/`Unclassified` row that turns out to hide
real incorrect behavior, or a security RTM `Vulnerable` finding — using this project's adaptation
of the discipline in `tdd-bdd-sbr.pdf` Appendix B: **interview for reproduction detail, reproduce,
root cause before any code change, failing tests before the fix, verification before closing, a
hidden-problem review, and a post-fix re-examination of whether the specification itself needs
updating.** Read `sbr/README.md` in full before the first execution step; its adapter section
names this project's test tiers, their directories, and their run commands — this skill never
hardcodes `go test` invocations that belong in the adapter.

Refinements this skill applies on top of the paper's literal cycle:

- **Interview before reproducing.** The paper assumes a reproduction is already in hand. This
  skill adds an explicit interview step (execution Step 2) whenever the target is a reported
  symptom with insufficient detail to actually replicate it — exact trigger steps, inputs,
  preconditions, and expected-vs-actual are gathered before any test is attempted, rather than
  guessing at a reproduction that might not represent the real bug.
- **Reproduce before diagnosing — a deliberate reordering of the paper's own step sequence.**
  Appendix B's literal order runs root-cause analysis (its Step 1) before any test is written (its
  Step 2). This skill inverts that: the reproduction test is written and run *first* (execution
  Step 3 below), and root-cause analysis (execution Step 4, still labeled "Appendix B Step 1" for
  traceability to the source) reasons from that test's concrete pass/fail result instead of from
  code-reading alone. See `sbr/README.md` → "Bug-fix step order: reproduce before diagnose" for
  why this project runs it this way.
- **Reproduce at the level the bug was reported, not at whatever level is most convenient.** A
  bug reported as observed behavior (a wrong RPC response, a symptom a user or QA pass hit) must
  be reproduced by a test at that same behavioral level — otherwise "fixed" only means "a unit
  passes," not "the reported symptom is gone." Once reproduced, if the root cause is fully and
  precisely expressible at a lower tier (typically L1/unit), that lower-tier test is the primary
  regression lock and additional mid-tier tests are optional, not mandatory padding.
- **Route the fix's traceability record by what the root cause actually is**, not just its test
  tier: a generalizable missing requirement updates `spec.md` and its feature RTM; a corner case
  or a pure architecture/implementation gap goes in the dedicated bug-fix RTM instead — matched by
  the glob `sbr/rtm/bugfix-*.rtm.md`, not a fixed filename (currently one file,
  `sbr/rtm/bugfix-general.rtm.md`). See `sbr/README.md` → "Bug-fix traceability: spec update vs.
  bug-fix RTM" for the full rule this skill follows.
- **Re-examine spec/RTM completeness after the fix, not only before.** Root-cause analysis
  (execution Step 4) makes its missing-requirements call before the fix exists. Execution Step 10
  revisits that call once the fix is built and verified, because a bug's true cause — a spec that
  was ambiguous or under-detailed rather than simply "already correct" — is sometimes only
  legible in hindsight, after seeing what the correct behavior actually had to be.

This is **not** an audit. `sbr-audit` and `sbr-security-audit` are strictly
read-only and document gaps without touching code. This skill is the deliberate, code-writing
complement: it exists specifically to close one gap those audits found, or to fix a bug reported
some other way. Running it is a real, reversible-but-consequential change to test files and
possibly application code — treat it with the same care as any other code-writing session, not
as a report-generation step.

This skill is also **not** the tool for a deliberate change to existing intended behavior — a
feature *upgrade* (spec says the behavior should now work differently, no defect involved).
That has its own skill, `sbr-feature-upgrade`, because its trigger (a planned decision,
not a diagnosed symptom) and traceability (always the feature RTM, never the bug-fix RTM) are
structurally different from a bugfix's. See that skill's Goal for the boundary between the two,
and `sbr/README.md` → "Feature extension vs. feature upgrade vs. bugfix" for the taxonomy.

## Operating Constraints

**This skill DOES modify test files and application code — that is its purpose.** It still has
hard limits:

- **Never guess a reproduction from an under-specified report.** If the interview (execution
  Step 2) cannot establish exact trigger steps, inputs, and expected-vs-actual behavior, do not
  write a reproduction test anyway and hope it represents the real bug — STOP and report that the
  target cannot be reproduced with the information available.
- **Never skip the red step.** Every test named in execution Steps 3 and 6 must be run and
  observed failing (for the right reason — the behavior's absence, not a config/compile error)
  before any implementation change is made. If a test can't be made to fail against current code,
  it is not testing the defect and must be rewritten, not accepted.
- **Never modify an existing test to accommodate a fix without stopping to report it first.** A
  fix that requires changing a pre-existing test's expected behavior is either revealing that the
  old test encoded the bug as "correct," or the new understanding is wrong — either way, this is
  a decision for a human, not something to resolve silently. Report the conflict and the two
  readings; do not edit the existing test until the user confirms which one is right.
- **The existing test suite must stay green after the fix.** Any existing test that starts
  failing after the implementation change is a regression signal, not noise to route around.
- **Root-cause analysis (Appendix B Step 1 / execution Step 4) makes no code changes** beyond the
  reproduction test already written in execution Step 3. It is diagnostic only — reading code,
  reading tests, reading the relevant RTM row if one exists, and reading the Step 3 test's
  result. Resist the pull to "just fix it while I'm looking" before the category is named and the
  root-cause-specific tests exist.
- **Minimum fix only.** Implement what the root-cause category and the failing tests require —
  no drive-by refactors, no unrelated cleanup, no expanding scope to adjacent code that isn't
  implicated by the actual root cause. Adjacent concerns surfaced during the hidden-problem review
  (execution Step 9) are reported as candidates for a follow-up, not folded into this fix
  automatically.
- **Never assert red/green from reading code.** Every red and every green claim in this skill's
  output must come from actually having run the test command named in the adapter and read its
  output — not from inspecting the test source and reasoning about what it "should" do.
- **Execution Step 10 always runs and always states a conclusion.** Unlike execution Step 5
  (which can be skipped outright when its trigger condition isn't met), Step 10 is never silently
  skipped — even when it finds nothing beyond what Step 4 already identified, it must say so
  explicitly.
- **RTM updates are scoped to the row(s) this fix actually closes**, and go in the file the root
  cause actually belongs to (see Step 11) — never rewrite an entire RTM file as a side effect of
  one bug fix; that is `sbr-audit`'s and `sbr-security-audit`'s job, run separately
  if a full re-audit is wanted afterward. This skill only ever appends/updates the specific
  row(s) for the defect it just closed, in exactly one of: an existing feature RTM row, a new
  feature RTM row (paired with a `spec.md` change), or the bug-fix RTM.
- **`spec.md` may be edited, but only for a generalizable missing-requirement root cause
  (identified in Step 4 or, in hindsight, in Step 10), and only the specific FR(s)/Acceptance
  Scenario(s) implicated.** This is a deliberate difference from `sbr-audit`/
  `sbr-security-audit`, which never touch `spec.md` — a bug fix that proves a requirement was
  actually missing has to correct the record, not just patch around it. Never touch unrelated
  parts of `spec.md`, and never add a new FR for a defect judged to be a corner case (that goes in
  the bug-fix RTM instead, per Step 5).

## Execution Steps

### 1. Resolve the target and load context

Parse `$ARGUMENTS` into one of: an `FR-XXX` ID, an `AV-ID`, a `file:line` pointer, or a free-text
bug description. Also capture **how the bug was reported** — the exact symptom, at whatever level
it was observed (an RPC call that returned the wrong thing, a UI-visible behavior, a specific
function returning a bad value). This "reporting level" drives Steps 2-3 below and must not be
lost by jumping straight to code.

- If it's an `FR-XXX` ID: locate the owning `specs/<slug>/spec.md` and its `sbr/rtm/<slug>.rtm.md`
  row; read the FR text and the row's existing Notes for what's already known about the gap.
- If it's an `AV-ID`: locate its row in the security RTM named in the adapter's "Security audit
  inputs" subsection; read the Attack Vector, existing evidence columns, and Notes.
- If it's a `file:line` or free-text description: read the surrounding code directly; check
  whether any RTM already references it (grep the ID pattern and nearby function names across
  `sbr/rtm/`) so a pre-existing finding isn't duplicated blind.
- If the target cannot be resolved to a specific, single piece of code (ambiguous description
  matching multiple candidates, or an ID that doesn't exist in any RTM), STOP and ask rather than
  guessing which one was meant.

Read `sbr/README.md`'s adapter section for the tier→directory→run-command mapping and search
conventions before continuing — this skill uses the same conventions the audit skills use to find
existing tests, plus the run commands to actually execute them.

### 2. Interview for reproduction detail

**Trigger.** Whenever the target carries a reported symptom — a free-text bug description, a
`file:line` pointer accompanied by a description, or an `FR-XXX`/`AV-ID` whose RTM row already
notes a known symptom — check whether Step 1's captured reporting level already contains enough
detail to write a reproduction test. A terse initial report ("X is broken," "returns the wrong
value sometimes") almost never does. **Skip this step entirely** when the target is a pure RTM
`Gap`/`Unclassified` row with no reported symptom at all — there is no bug being described, only
a coverage gap, and Step 3 will directly attempt the FR's own assertion instead.

**Minimum bar to proceed past this step.** The reproduction detail must include, at minimum:

- **Exact trigger steps or call sequence** — the specific RPC/endpoint/UI action(s) and their
  order, not a paraphrase ("calling `CreateRentalRequest` then `ApproveRentalRequest`," not
  "using the rental flow").
- **Exact input values, or a precise characterization of them** — the request payload/parameters,
  or the specific data condition (a particular org's membership state, a specific record, a
  specific role) — not "some data."
- **Preconditions/environment** — caller role, org/account state, relevant feature-flag state,
  and which environment (local/precommit/staging) — anything the behavior might depend on.
- **Expected vs. actual** — what should have happened per the reporter's understanding, and what
  actually happened, verbatim where available (exact error message, wrong value, wrong status
  code — not a paraphrase of the error).
- **Frequency/consistency** — always reproduces, or intermittent (and under what pattern, if
  known).

Ask for whichever of these Step 1's capture is missing, as one consolidated set of questions
rather than one-at-a-time back-and-forth. "Unknown" is an acceptable answer for frequency — accept
it explicitly rather than blocking on it. **The first three items (trigger steps, inputs,
expected-vs-actual) are not optional**: do not proceed to Step 3 without them. If they genuinely
cannot be supplied, STOP and report that this target cannot be reproduced with the information
available, rather than guessing at a reproduction test that might not represent the real bug.

Record the final, consolidated reproduction detail for inclusion in the Step 13 report, so it
survives even if root-cause analysis later runs long.

### 3. Reproduce the symptom

Write a test at the same behavioral level the bug was actually reported at (per Steps 1-2's
captured reporting level; per the adapter's tier mapping — often integration or e2e, since that's
how most bugs are reported: as a wrong response from a call, not as a wrong return value from an
internal function). Run it via the adapter's run command.

- **If it fails:** the reported symptom is confirmed real and reproducible. Proceed to Step 4
  with this concrete failure in hand — root-cause analysis should explain exactly why *this*
  assertion fails, not just characterize the area generally.
- **If it passes immediately:** for a target with no reported symptom at all (an RTM `Gap`/
  `Unclassified` row — only a coverage gap, no claimed bug), this is the expected default
  outcome; proceed to Step 4 to confirm via code reading whether the behavior is actually
  correct. For a target that *was* reported as an observed symptom, an immediate pass means
  either the reporting level was wrong or the bug doesn't reproduce as described — do not
  silently treat this as "no bug." Say so explicitly and reconsider Step 1's captured reporting
  level and Step 2's interview answers (a different level, a missing precondition, a stale repro)
  before concluding there's nothing here.

This test is not optional regardless of what Step 4 later concludes: it is the proof the actual
reported problem existed (or, for a coverage-gap target, the first real evidence either way) and,
later — after the fix — that it's gone.

### 4. Step 1 (Appendix B) — Root cause analysis

Diagnostic only — no code changes in this step, and no test changes beyond the reproduction test
already written in Step 3. Read the implicated code path end to end, from the entry point down
through whatever layers actually handle it in this codebase (e.g. handler → service → repository
for a backend endpoint; UI/ViewModel → repository/storage for a client), using Step 3's concrete
pass/fail result as the starting evidence rather than reading cold, and map the defect to exactly
one category:

- **Missing requirements** — the behavior was never specified anywhere (no Acceptance Scenario in
  the owning `spec.md` covers it). The system was never told what to do here. Additionally judge:
  is this **generalizable** (any future implementer of this feature would need to know the rule,
  not just this one caller/scenario), or a **corner case** (a narrow edge condition that doesn't
  warrant a standing rule in `spec.md`)? This judgment determines the traceability route in
  Step 5/Step 11 — state it explicitly, don't leave it implicit.
- **Architecture design gap** — the behavior was intended and even partly specified, but the
  service/interface boundary doesn't support it correctly (this project's L2/L3 tiers). Normally a
  corner-case-RTM target, never a `spec.md` edit here in Step 5 — but revisit this call in Step 10
  once the fix exists, since a spec that only *looked* already-correct can still turn out to have
  been ambiguous.
- **Implementation design gap** — the behavior was specified and the architecture supports it,
  but the method-level logic is wrong or incomplete (this project's L1 tier). Same routing as
  architecture design gap, and the same Step 10 revisit applies.

State the category and the reasoning, citing file:line for the specific missing/wrong logic —
ideally by contrasting with a sibling function that gets the equivalent case right (the same
discipline the security audit skill uses; it's the strongest evidence something is a bug rather
than a deliberate design choice). If the investigation reveals there is in fact no bug — existing
behavior is already correct and Step 3's immediate pass was the correct result, the RTM row was a
pure test-coverage gap — say so explicitly and continue to Step 6 with "no fix; only Step 3's
deferred dynamic genuineness check remains" as the plan; not every RTM `Gap` hides a defect (see
`sbr/remediation-plan.md`'s own results: most Phase 1-4 items closed with "No bug found").

### 5. If missing requirements and generalizable: update the spec first

Only when Step 4 concluded **missing requirements** *and* judged it **generalizable**. Add or
amend the specific `FR-XXX` (and its Acceptance Scenario) in the owning `spec.md` — nothing else
in that file. This must happen before Step 6, so any additional root-cause-isolating test written
next is derived from a complete specification, not from an implicit understanding of what should
have been written. State the exact diff (old FR text if amended, new FR text) in the report. If
Step 4 instead judged the missing-requirement gap a corner case, or categorized it as an
architecture/implementation gap, skip this step entirely — `spec.md` is not touched here (Step 10
may still revisit that call later, with the benefit of hindsight), and Step 11 will route the fix
to the bug-fix RTM instead unless Step 10 changes that.

### 6. Step 2 (Appendix B) — Write root-cause-specific tests

Using Step 4's diagnosed category, add whatever test(s) are needed to precisely isolate the root
cause — beyond Step 3's reporting-level reproduction, a test that pins the exact defective logic.
If the root cause is fully and precisely expressible at a lower tier than the reproduction test
(typically L1/unit — e.g. a missing status guard, a wrong comparison), add that tier's test too;
it gives the tightest, fastest-running regression lock. Once the reproduction test and, where
applicable, the root-cause-isolating test (lowest precise tier) are both red for the diagnosed
reason, **additional intermediate tiers are optional** — do not mechanically add an L2 test
between an L1 isolation test and an L3 reproduction test just to fill the pyramid. If a tier is
skipped, note briefly why the two tests already written are sufficient (mirrors the "justify
every blank cell" discipline from the project's own RTM hardening pass).

**Every test named here, and the Step 3 reproduction test, must fail before the fix** when a real
bug was diagnosed in Step 4 — this is the literal requirement, not optional padding. A test that
doesn't fail here either isn't testing the diagnosed root cause (rewrite it) or means Step 4's
diagnosis was wrong (return to Step 4 — do not force a red result by weakening the assertion). If
Step 4 concluded there is no bug, no new test is required by this step beyond what Step 3 already
wrote; proceed straight to the genuineness check below.

**Genuineness check — required before any test from Step 3 or this step can be used as evidence
in Step 11.** See `sbr/README.md` → "Posthoc test-genuineness check" for the full rationale and
procedure.

- **Static check (Steps A-C), every test.** Derive what it should assert from the FR/AV/spec text
  alone, before re-reading the test's own assertions closely, then classify Spec-Aligned /
  Implementation-Coupled / Vacuous. Anything but Spec-Aligned must be rewritten against the
  independently derived expectation before continuing, even if it already went red.
- **SBR-Trace annotation, every test written or touched in Step 3 or this step.** Add or update
  the `// SBR-Trace: <ID> — <behavior>` comment directly above the test function (see
  `sbr/README.md` → "SBR-Trace test annotations" for the convention and this project's adapter
  for the exact Go comment format), using Step A's independently derived expectation as its
  content — this persists that derivation instead of discarding it, so a future audit or bugfix
  run reads a checkable claim instead of re-deriving it. If a pre-existing sibling test in the
  same file already carries an annotation and this run's mutation check (below) proves it wrong
  (stale relative to what the test actually does now), correct that annotation too as part of
  this step — do not leave a known-stale claim in place.
- **Dynamic check (Step D), with a broadened blast radius.** See `sbr/README.md` → "Mutation
  blast radius (extending Step D)" for the full procedure; summary: for the Step 3 reproduction
  test, if it failed there against a real bug, that failure already *is* Step D — no separate
  mutation needed. If it instead passed immediately in Step 3 (the no-bug branch, now confirmed
  by Step 4's code reading), that pass has not yet proven anything: temporarily break the
  specific behavior it claims to assert, then re-run **the target test together with its sibling
  tests in the same file** (and any other test elsewhere annotated with the same `FR-XXX`/
  `AV-ID`) via the adapter's run command — not the target alone. Confirm the target now fails for
  that reason, then revert and confirm green again — do this now, before proceeding to Step 7.
  Any test newly written in this step already clears Step D by construction (the "must fail
  before the fix" requirement above *is* its dynamic check, when a real bug was diagnosed), but
  still run it alongside its siblings under the same mutation for the blast-radius check.
  Interpret sibling results per `sbr/README.md`'s procedure: a sibling claiming this coverage
  that stays green is a newly discovered vacuous test (report as a follow-up candidate per Step
  9's discipline, don't silently expand this run's fix scope to cover it); a sibling with no
  claimed coverage that flips anyway is a coupling signal worth noting, not an automatic defect.

### 7. Step 3 (Appendix B) — Implement the fix

If Step 4 concluded there is no bug and every test cleared its genuineness check in Step 6, there
is no implementation change to make — proceed to Step 8 with those tests as the only artifact.

Otherwise, implement the minimum change that makes the new test(s) pass, scoped exactly to what
the root-cause category names. Do not touch any test written in Step 3 or Step 6 to make it pass
— if a test itself turns out to have been wrong, stop and report that per the Operating
Constraints above rather than adjusting it unilaterally.

### 8. Step 4 (Appendix B) — Verify

Run the new test(s) again and confirm green. Then run the full existing suite for every tier
touched (at minimum the tier(s) of the fix; prefer running unit+integration+e2e together via the
adapter's combined command when the fix touches shared code) and confirm nothing that was passing
before now fails. Any newly-failing existing test is a regression — stop and resolve it (by
correcting the fix, not by touching the existing test) before proceeding.

Confirm the *originally reported symptom* is gone, not just that the new tests are green — the
completion bar is "tests succeed **and** the bug condition does not recur," not tests-green
alone. Re-running Step 3's reproduction test as part of "the new test(s)" above satisfies this
for anything captured by an automated test; if the bug was also reported through a channel not
fully captured by one (e.g. a manual QA repro step noted in Step 2's interview), re-check that
channel too where feasible and say so explicitly in the report.

Unless the adapter names a staging/production environment this skill can actually reach, "verify
in the live system" (Appendix B Step 4's deployment-verification instruction) is satisfied by the
full local suite passing against real local infrastructure (e.g. a local database instance and a
running local server, per whatever the adapter's L2/L3 run commands stand up) — not by an actual
deployment.

### 9. Step 5 (Appendix B) — Review for hidden problems

Ask, and answer explicitly in the report:

- Given this root cause, what other code paths might share the same defect class? (Search for
  structurally similar functions — same pattern the security audit skill uses for "sibling
  endpoint" comparisons.)
- Are there adjacent scenarios in the same feature/RTM that should be strengthened now that this
  gap class is visible?
- Does this suggest a systematic weakness worth flagging in `sbr/remediation-plan.md` or a new
  RTM row, even if not fixed in this pass?

List findings as candidates for a follow-up fix (with enough detail — file:line, suggested
category — that a future run of this same skill could pick one up directly) rather than
automatically expanding this run's scope to cover them. If one is trivially the same one-line fix
applied twice (e.g. an identical missing guard in a true sibling function), it's reasonable to
include it in this same pass — say explicitly that this happened and why, don't silently widen
scope without noting it.

### 10. Re-examine specification/RTM completeness with hindsight

Step 4's missing-requirements-vs-already-correct-requirement call was made **before** the fix
existed, from reading code. This step reconsiders that call **after** the fix is implemented and
verified, and after Step 9's hidden-problem review — because a bug's true cause is sometimes only
fully legible once the correct behavior has actually been built, not before. This step always
runs and always states a conclusion; it is never silently skipped, even when Step 4 already
correctly identified everything there was to find.

Ask explicitly:

- Does the finished fix show that the owning FR's spec text — even one Step 4 judged "already
  correct" (an architecture or implementation design gap) — was actually ambiguous or
  under-detailed in a way that plausibly let this bug happen? A detail a future implementer would
  need and could not have gotten from the spec as currently worded?
- Did Step 9's hidden-problem review surface a systematic weakness that, in hindsight, is better
  understood as a missing or underspecified requirement rather than only a follow-up candidate?

**If yes to either** — this is a genuinely late-discovered finding, distinct from Step 5's early
conditional update (which fires only when Step 4's *initial* categorization was already "missing
requirements"). Judge generalizable vs. corner case exactly as Step 4 does, and:

- **Generalizable**: amend the specific FR/Acceptance Scenario in `spec.md` — nothing else —
  state the exact diff, and mark it explicitly in the report as **discovered post-fix**, not
  during initial root-cause analysis, so a future reader understands this wasn't foreseeable
  before the fix existed.
- **Corner case**: do not touch `spec.md`; capture the insight either in Step 9's hidden-problem
  findings or in the bug-fix RTM's Notes for this defect (Step 11) instead.

**If no** — state so explicitly ("re-examined; no additional spec/RTM discrepancy beyond Step 4's
finding, if any") rather than moving on silently.

Any `spec.md` amendment from this step changes which traceability home Step 11 routes to —
resolve this step before Step 11, not after.

### 11. Update traceability

Route the record to exactly one place, based on Steps 4, 5, and 10:

- **Target had a known `FR-ID` or `AV-ID`** (an existing RTM row): update only that row — flip
  the Boundary Status per the vocabulary in `sbr/README.md` (`Gap`→`Complete`, or for the
  security RTM, `Vulnerable`→`Complete` following the strikethrough-and-"Fixed" convention
  already used in `sbr/rtm/009-security.rtm.md`'s Executive Summary), cite the new test(s) by
  name/subtest per the adapter's search-evidence format, and note "genuineness: static +
  mutation-confirmed" (this skill always clears both checks per Step 6, so this level is
  constant here — the distinction only matters when comparing against a row an audit-only run
  classified). Do not touch any other row.
- **Step 5 fired, or Step 10 concluded a generalizable missing requirement** (either way,
  `spec.md` was updated): add the corresponding row to that feature's
  `sbr/rtm/<feature-slug>.rtm.md` for the new/amended FR, citing the new test(s), exactly as
  `sbr-audit` would for a freshly-closed gap. If Step 10 was the trigger, note in this
  row too that the requirement change was discovered post-fix.
- **None of the above** (corner case, or an architecture/implementation gap with no pre-existing
  RTM row, and Step 10 found nothing generalizable): resolve the target bug-fix RTM file by
  globbing `sbr/rtm/bugfix-*.rtm.md` — never a hardcoded filename, since a project may name or
  split this file differently over time:
  - **Zero matches**: initialize `sbr/rtm/bugfix-general.rtm.md` from
    `.specify/templates/bugfix-rtm-template.md` (fill in its bracketed placeholders from the
    adapter, per the template's own instructions) — never invent the file's schema ad hoc.
  - **Exactly one match**: use it.
  - **More than one match**: STOP and ask which file this defect belongs in, rather than
    guessing — do not default to the first alphabetically or most-recently-modified match.
  Then append one row: next `BUGFIX-NNN` ID sequential *within that file* (a second bugfix RTM,
  if one exists, restarts its own `BUGFIX-001`), the reported symptom, the root-cause category,
  the reproduction test and (if written) the root-cause-isolating test, the fix location, any
  Step 10 corner-case insight, and a one-line note. Update that file's Summary counts. Never
  rewrite existing rows, and never touch any other `bugfix-*.rtm.md` file that wasn't the
  resolved target.

Never write to more than one of these three homes for a single defect.

### 12. Handle "tests pass but the bug persists"

If, after Step 8's verification, the originally-reported symptom still reproduces (this only
applies when the target was a reported bug with an observable symptom, not a pure RTM gap), the
Step 4 root-cause analysis was wrong. Do not declare victory because tests are green. Report
explicitly: "the [category] hypothesis has been ruled out; ruled-out finding: [what Step 6's
tests now correctly cover]," then restart at Step 4 with that hypothesis excluded, carrying the
now-passing tests forward (they are not wasted — they permanently strengthen the trust boundary
even though they didn't locate the bug).

### 13. Report

Print a concise summary: the target and how it was reported, Step 2's consolidated reproduction
detail, the Step 3 reproduction result (failed naturally / passed immediately), the root-cause
category found (and generalizable-vs-corner-case judgment if missing-requirements), the test(s)
added (file, name, tier, and which was the reproduction test vs. the isolation test), the
genuineness-check outcome per test (Step 6 — Spec-Aligned confirmed, the SBR-Trace annotation
added/updated, and whether the dynamic check was the natural pre-fix red or a synthetic mutation,
including any test that had to be rewritten after an Implementation-Coupled/Vacuous
classification), Step 6's mutation blast-radius findings (any sibling test discovered vacuous or
unexpectedly coupled), whether a code fix was needed and where,
full-suite verification result including confirmation the original symptom no longer recurs,
hidden-problem review findings (fixed-in-this-pass vs. flagged-for-follow-up), Step 10's post-fix
specification/RTM re-examination outcome (any late-discovered spec gap, generalizable vs. corner
case, and where it was recorded), and exactly which traceability home was updated (feature RTM
row, new FR + feature RTM row, or bug-fix RTM row). If Step 12's restart loop triggered, report
the full chain of ruled-out hypotheses, not just the final one.

## Post-Execution Checks

After producing the result, check if `.specify/extensions.yml` exists in the project root.

- If it exists, read it and look for entries under the `hooks.after_sbr_bugfix` key
- If the YAML cannot be parsed or is invalid, skip hook checking silently and continue normally
- Filter out hooks where `enabled` is explicitly `false`. Treat hooks without an `enabled` field as enabled by default.
- For each remaining hook, do **not** attempt to interpret or evaluate hook `condition` expressions:
  - If the hook has no `condition` field, or it is null/empty, treat the hook as executable
  - If the hook defines a non-empty `condition`, skip the hook and leave condition evaluation to the HookExecutor implementation
- Report the fix outcome (target, root cause, tests added, suite verification result) before
  listing any hooks, so users can decide whether to run optional follow-up commands (e.g.
  re-running `sbr-audit`/`sbr-security-audit` to confirm the RTM reflects the fix
  project-wide, or dispatching a follow-up fix for a flagged hidden-problem candidate).
- When constructing slash commands from hook command names, replace dots (`.`) with hyphens (`-`).
- For each executable hook, output the same optional/mandatory blocks used in Pre-Execution
  Checks, substituting the after-hook framing, and actually invoke mandatory hooks before
  finishing.
- If no hooks are registered or `.specify/extensions.yml` does not exist, skip silently
