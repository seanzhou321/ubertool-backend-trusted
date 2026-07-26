---
name: "speckit-sbr-bugfix"
description: "Fix a single bug, RTM gap, or security finding using the SBR framework's Appendix B bug-fix discipline: diagnostic root-cause analysis before any code change, failing tests before the fix, and a hidden-problem review after."
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
real incorrect behavior, or a security RTM `Vulnerable` finding — using the root-cause-first
discipline in `tdd-bdd-sbr.pdf` Appendix B: **root cause before tests, tests before the fix,
verification before closing, and a hidden-problem review before calling it done.** Read
`sbr/README.md` in full before the first execution step; its adapter section names this
project's test tiers, their directories, and (as of this skill's introduction) their run
commands — this skill never hardcodes `go test` invocations that belong in the adapter.

Two refinements this skill applies on top of the paper's literal cycle, both driven by how bugs
actually surface in this project:

- **Reproduce at the level the bug was reported, not at whatever level is most convenient.** A
  bug reported as observed behavior (a wrong RPC response, a symptom a user or QA pass hit) must
  first be reproduced by a test at that same behavioral level — otherwise "fixed" only means "a
  unit passes," not "the reported symptom is gone." Once reproduced, if the root cause is fully
  and precisely expressible at a lower tier (typically L1/unit), that lower-tier test is the
  primary regression lock and additional mid-tier tests are optional, not mandatory padding.
- **Route the fix's traceability record by what the root cause actually is**, not just its test
  tier: a generalizable missing requirement updates `spec.md` and its feature RTM; a corner case
  or a pure architecture/implementation gap goes in the dedicated bug-fix RTM instead — matched by
  the glob `sbr/rtm/bugfix-*.rtm.md`, not a fixed filename (currently one file,
  `sbr/rtm/bugfix-general.rtm.md`). See `sbr/README.md` → "Bug-fix traceability: spec update vs.
  bug-fix RTM" for the full rule this skill follows.

This is **not** an audit. `speckit-sbr-audit` and `speckit-security-audit` are strictly
read-only and document gaps without touching code. This skill is the deliberate, code-writing
complement: it exists specifically to close one gap those audits found, or to fix a bug reported
some other way. Running it is a real, reversible-but-consequential change to test files and
possibly application code — treat it with the same care as any other code-writing session, not
as a report-generation step.

## Operating Constraints

**This skill DOES modify test files and application code — that is its purpose.** It still has
hard limits:

- **Never skip the red step.** A new test must be run and observed failing (for the right
  reason — the behavior's absence, not a config/compile error) before any implementation change
  is made. If a test can't be made to fail against current code, it is not testing the gap and
  must be rewritten, not accepted.
- **Never modify an existing test to accommodate a fix without stopping to report it first.** A
  fix that requires changing a pre-existing test's expected behavior is either revealing that the
  old test encoded the bug as "correct," or the new understanding is wrong — either way, this is
  a decision for a human, not something to resolve silently. Report the conflict and the two
  readings; do not edit the existing test until the user confirms which one is right.
- **The existing test suite must stay green after the fix.** Any existing test that starts
  failing after the implementation change is a regression signal, not noise to route around.
- **Root-cause analysis (Appendix B Step 1 / execution Step 2) makes no code changes.** It is
  diagnostic only — reading code, reading tests, reading the relevant RTM row if one exists.
  Resist the pull to "just fix it while I'm looking" before the category is named and the tests
  exist.
- **Minimum fix only.** Implement what the root-cause category and the new failing tests require
  — no drive-by refactors, no unrelated cleanup, no expanding scope to adjacent code that isn't
  implicated by the actual root cause. Adjacent concerns surfaced during the hidden-problem review
  (execution Step 7) are reported as candidates for a follow-up, not folded into this fix
  automatically.
- **Never assert red/green from reading code.** Every red and every green claim in this skill's
  output must come from actually having run the test command named in the adapter and read its
  output — not from inspecting the test source and reasoning about what it "should" do.
- **RTM updates are scoped to the row(s) this fix actually closes**, and go in the file the root
  cause actually belongs to (see Step 8) — never rewrite an entire RTM file as a side effect of
  one bug fix; that is `speckit-sbr-audit`'s and `speckit-security-audit`'s job, run separately
  if a full re-audit is wanted afterward. This skill only ever appends/updates the specific
  row(s) for the defect it just closed, in exactly one of: an existing feature RTM row, a new
  feature RTM row (paired with a `spec.md` change), or the bug-fix RTM.
- **`spec.md` may be edited, but only for a generalizable missing-requirement root cause, and
  only the specific FR(s)/Acceptance Scenario(s) implicated.** This is a deliberate difference
  from `speckit-sbr-audit`/`speckit-security-audit`, which never touch `spec.md` — a bug fix that
  proves a requirement was actually missing has to correct the record, not just patch around it.
  Never touch unrelated parts of `spec.md`, and never add a new FR for a defect judged to be a
  corner case (that goes in the bug-fix RTM instead, per Step 3).

## Execution Steps

### 1. Resolve the target and load context

Parse `$ARGUMENTS` into one of: an `FR-XXX` ID, an `AV-ID`, a `file:line` pointer, or a free-text
bug description. Also capture **how the bug was reported** — the exact symptom, at whatever level
it was observed (an RPC call that returned the wrong thing, a UI-visible behavior, a specific
function returning a bad value). This "reporting level" drives Step 4 below and must not be lost
by jumping straight to code.

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

### 2. Step 1 (Appendix B) — Root cause analysis

Diagnostic only — no code changes in this step. Read the implicated code path end to end, from
the entry point down through whatever layers actually handle it in this codebase (e.g. handler
→ service → repository for a backend endpoint; UI/ViewModel → repository/storage for a client),
and map the defect to exactly one category:

- **Missing requirements** — the behavior was never specified anywhere (no Acceptance Scenario in
  the owning `spec.md` covers it). The system was never told what to do here. Additionally judge:
  is this **generalizable** (any future implementer of this feature would need to know the rule,
  not just this one caller/scenario), or a **corner case** (a narrow edge condition that doesn't
  warrant a standing rule in `spec.md`)? This judgment determines the traceability route in
  Step 3/Step 8 — state it explicitly, don't leave it implicit.
- **Architecture design gap** — the behavior was intended and even partly specified, but the
  service/interface boundary doesn't support it correctly (this project's L2/L3 tiers). Always a
  corner-case-RTM target, never a `spec.md` edit — the requirement was already correct.
- **Implementation design gap** — the behavior was specified and the architecture supports it,
  but the method-level logic is wrong or incomplete (this project's L1 tier). Same routing as
  architecture design gap: bug-fix RTM, not `spec.md`.

State the category and the reasoning, citing file:line for the specific missing/wrong logic —
ideally by contrasting with a sibling function that gets the equivalent case right (the same
discipline the security audit skill uses; it's the strongest evidence something is a bug rather
than a deliberate design choice). If the investigation reveals there is in fact no bug — existing
behavior is already correct and the RTM row was a pure test-coverage gap — say so explicitly and
skip to Step 4 with "write the missing test only" as the plan; not every RTM `Gap` hides a defect
(see `sbr/remediation-plan.md`'s own results: most Phase 1-4 items closed with "No bug found").

### 3. If missing requirements and generalizable: update the spec first

Only when Step 2 concluded **missing requirements** *and* judged it **generalizable**. Add or
amend the specific `FR-XXX` (and its Acceptance Scenario) in the owning `spec.md` — nothing else
in that file. This must happen before Step 4, so the failing test written next is derived from a
complete specification, not from an implicit understanding of what should have been written.
State the exact diff (old FR text if amended, new FR text) in the report. If Step 2 instead
judged the missing-requirement gap a corner case, or categorized it as an architecture/
implementation gap, skip this step entirely — `spec.md` is not touched, and Step 8 will route the
fix to the bug-fix RTM instead.

### 4. Step 2 (Appendix B) — Write failing tests

**First, reproduce at the reporting level captured in Step 1.** Write a test at the same
behavioral level the bug was actually observed at (per the adapter's tier mapping — this is
often integration or e2e, since that's how most bugs are reported: as a wrong response from a
call, not as a wrong return value from an internal function). Run it via the adapter's run
command and confirm it **fails for the right reason** — reproducing the reported symptom, not a
typo or fixture error. This test is not optional regardless of root-cause category: it is the
proof the actual reported problem existed and, later, that it's gone.

**Then, only if it adds isolation value:** if the root cause is fully and precisely expressible
at a lower tier (typically L1/unit — e.g. a missing status guard, a wrong comparison), add that
tier's test too, since it gives the tightest, fastest-running regression lock. Once the
reproduction test (behavioral level) and, where applicable, the root-cause-isolating test (lowest
precise tier) are both red for the right reason, **additional intermediate tiers are optional**
— do not mechanically add an L2 test between an L1 isolation test and an L3 reproduction test just
to fill the pyramid. If a tier is skipped, note briefly why the two tests already written are
sufficient (mirrors the "justify every blank cell" discipline from the project's own RTM
hardening pass).

If Step 2 concluded there is no bug, the reproduction test may pass immediately against current
code — that is a valid outcome (the RTM's Boundary Status was `Gap`, not `Vulnerable`/incorrect
behavior); do not force a red result that isn't real.

### 5. Step 3 (Appendix B) — Implement the fix

If every test written in Step 4 passed immediately, there is no implementation change to make —
proceed to Step 6 with those tests as the only artifact.

Otherwise, implement the minimum change that makes the new test(s) pass, scoped exactly to what
the root-cause category names. Do not touch any test written in Step 4 to make it pass — if a
test itself turns out to have been wrong, stop and report that per the Operating Constraints
above rather than adjusting it unilaterally.

### 6. Step 4 (Appendix B) — Verify

Run the new test(s) again and confirm green. Then run the full existing suite for every tier
touched (at minimum the tier(s) of the fix; prefer running unit+integration+e2e together via the
adapter's combined command when the fix touches shared code) and confirm nothing that was passing
before now fails. Any newly-failing existing test is a regression — stop and resolve it (by
correcting the fix, not by touching the existing test) before proceeding.

Unless the adapter names a staging/production environment this skill can actually reach, "verify
in the live system" (Appendix B Step 4's deployment-verification instruction) is satisfied by the
full local suite passing against real local infrastructure (e.g. a local database instance and a
running local server, per whatever the adapter's L2/L3 run commands stand up) — not by an actual
deployment.

### 7. Step 5 (Appendix B) — Review for hidden problems

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

### 8. Update traceability

Route the record to exactly one place, based on Steps 2-3:

- **Target had a known `FR-ID` or `AV-ID`** (an existing RTM row): update only that row — flip
  the Boundary Status per the vocabulary in `sbr/README.md` (`Gap`→`Complete`, or for the
  security RTM, `Vulnerable`→`Complete` following the strikethrough-and-"Fixed" convention
  already used in `sbr/rtm/009-security.rtm.md`'s Executive Summary), cite the new test(s) by
  name/subtest per the adapter's search-evidence format. Do not touch any other row.
- **Step 3 fired** (generalizable missing requirement, `spec.md` updated): add the corresponding
  row to that feature's `sbr/rtm/<feature-slug>.rtm.md` for the new/amended FR, citing the new
  test(s), exactly as `speckit-sbr-audit` would for a freshly-closed gap.
- **Neither of the above** (corner case, or an architecture/implementation gap with no
  pre-existing RTM row): resolve the target bug-fix RTM file by globbing `sbr/rtm/bugfix-*.rtm.md`
  — never a hardcoded filename, since a project may name or split this file differently over time:
  - **Zero matches**: initialize `sbr/rtm/bugfix-general.rtm.md` from
    `.specify/templates/bugfix-rtm-template.md` (fill in its bracketed placeholders from the
    adapter, per the template's own instructions) — never invent the file's schema ad hoc.
  - **Exactly one match**: use it.
  - **More than one match**: STOP and ask which file this defect belongs in, rather than
    guessing — do not default to the first alphabetically or most-recently-modified match.
  Then append one row: next `BUGFIX-NNN` ID sequential *within that file* (a second bugfix RTM,
  if one exists, restarts its own `BUGFIX-001`), the reported symptom, the root-cause category,
  the reproduction test and (if written) the root-cause-isolating test, the fix location, and a
  one-line note. Update that file's Summary counts. Never rewrite existing rows, and never touch
  any other `bugfix-*.rtm.md` file that wasn't the resolved target.

Never write to more than one of these three homes for a single defect.

### 9. Handle "tests pass but the bug persists"

If, after Step 6's verification, the originally-reported symptom still reproduces (this only
applies when the target was a reported bug with an observable symptom, not a pure RTM gap), the
Step 2 root-cause analysis was wrong. Do not declare victory because tests are green. Report
explicitly: "the [category] hypothesis has been ruled out; ruled-out finding: [what Step 4's
tests now correctly cover]," then restart at Step 2 with that hypothesis excluded, carrying the
now-passing tests forward (they are not wasted — they permanently strengthen the trust boundary
even though they didn't locate the bug).

### 10. Report

Print a concise summary: the target and how it was reported, the root-cause category found (and
generalizable-vs-corner-case judgment if missing-requirements), the test(s) added (file, name,
tier, and which was the reproduction test vs. the isolation test), whether a code fix was needed
and where, full-suite verification result, hidden-problem review findings (fixed-in-this-pass vs.
flagged-for-follow-up), and exactly which traceability home was updated (feature RTM row,
new FR + feature RTM row, or bug-fix RTM row). If Step 9's restart loop triggered, report the
full chain of ruled-out hypotheses, not just the final one.

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
  re-running `speckit-sbr-audit`/`speckit-security-audit` to confirm the RTM reflects the fix
  project-wide, or dispatching a follow-up fix for a flagged hidden-problem candidate).
- When constructing slash commands from hook command names, replace dots (`.`) with hyphens (`-`).
- For each executable hook, output the same optional/mandatory blocks used in Pre-Execution
  Checks, substituting the after-hook framing, and actually invoke mandatory hooks before
  finishing.
- If no hooks are registered or `.specify/extensions.yml` does not exist, skip silently
