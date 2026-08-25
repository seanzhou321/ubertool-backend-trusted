---
name: "sbr-feature-upgrade"
description: "Change existing intended behavior under true pre-implementation gating: update the requirement, design, and tests first, confirm the updated tests fail against the old implementation, then implement."
argument-hint: "<FR-ID> [description of the behavior change] — required, an existing requirement whose intended behavior is changing"
compatibility: "Requires spec-kit project structure with sbr/README.md's adapter"
metadata:
  author: "sbr-extension"
  source: "sbr/README.md -> \"Feature extension vs. feature upgrade vs. bugfix\"; gating discipline adapted from tdd-bdd-sbr.pdf"
user-invocable: true
disable-model-invocation: false
---


## User Input

```text
$ARGUMENTS
```

You **MUST** consider the user input before proceeding. This skill requires a single, concrete
target: an existing `FR-XXX` whose *intended* behavior is deliberately changing. If `$ARGUMENTS`
is empty, STOP and ask what requirement is changing and how, rather than guessing — like
`sbr-bugfix`, this skill writes code and edits `spec.md`, so it must never invent its own
scope.

## Pre-Execution Checks

**Check for extension hooks (before feature upgrade)**:

- Check if `.specify/extensions.yml` exists in the project root.
- If it exists, read it and look for entries under the `hooks.before_sbr_feature_upgrade` key
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

Change **existing, already-intended** behavior — the requirement itself is deliberately changing,
not just being discovered wrong — under true pre-implementation gating: update the requirement,
design, and tests first, confirm the updated tests fail against the *old* implementation, then
implement. See `sbr/README.md` → "Feature extension vs. feature upgrade vs. bugfix" for the
taxonomy this skill fills one leg of, and why gating is trustworthy here specifically: the current
implementation is a known, working, previous-requirement baseline, so a test asserting the new
behavior can be run against it and trusted to go red for a real reason — unlike feature extension,
where nothing exists yet to be red against.

**This skill is not for:**

- **Brand-new behavior with no existing `FR-XXX` to amend** — new classes/packages/RPCs with
  nothing prior to contrast against. That is a feature *extension*, handled by ordinary
  `speckit-implement` under the posthoc genuineness check (`sbr/README.md` → "Posthoc
  test-genuineness check"), not this skill.
- **An undiagnosed symptom or coverage gap** — nobody decided the requirement should change;
  something is (or might be) already wrong relative to intent that was never properly verified.
  That is a bugfix, handled by `sbr-bugfix`, which performs a diagnostic root-cause step
  this skill deliberately never does. The dividing question: *did anyone decide this should
  change, or did we just discover it's wrong?* A deliberate decision is an upgrade; a discovery
  is a bugfix, even if the fix also ends up touching `spec.md`.

If `$ARGUMENTS` turns out to name behavior with no existing FR at all, or an apparent defect
nobody decided to change on purpose, STOP and say which of the other two skills fits instead
rather than forcing this skill's gating process onto the wrong kind of change.

## Operating Constraints

**This skill DOES modify `spec.md`, design artifacts, test files, and application code — that is
its purpose.** It still has hard limits:

- **`spec.md` (and any affected design artifact) MUST be updated first, unconditionally** — this
  is a deliberate contrast with `sbr-bugfix`, where the spec update is conditional on the
  root-cause category. Here it is never conditional: a feature upgrade *is* a requirement change
  by definition, so Step 2 always runs.
- **Never skip the red step, and never fake it with a synthetic mutation.** The updated/new
  tests must be run against the current, pre-upgrade implementation and observed failing for a
  real reason before any implementation change is made. Because the old implementation is a
  trustworthy known baseline here, this skill does not use the posthoc dynamic-genuineness
  mutation substitute (`sbr/README.md`'s Step D) — a real red against real old code is always
  available and always required.
- **Modifying an existing test's assertions to reflect the new behavior is expected here, not a
  red flag** — the deliberate opposite of `sbr-bugfix`'s rule against silently changing
  an existing test. Record the exact diff (old assertion → new assertion) for every test touched
  this way, so the change stays auditable.
- **The rest of the existing suite must stay green after the change.** Any test outside the ones
  deliberately updated in Step 3 that starts failing is an unintended regression — a caller or
  scenario this upgrade wasn't supposed to affect — and must be resolved, not waved through as an
  accepted consequence of the upgrade.
- **Minimum implementation change only**, scoped to exactly what the updated spec/design
  describes. No drive-by refactors, no unrelated cleanup.
- **Never assert red/green from reading code.** Every red and every green claim must come from
  actually running the adapter's test commands.
- **RTM updates always target the feature RTM row for the changed FR.** Never the bug-fix RTM —
  a feature upgrade is never a corner case by this skill's own definition; if Step 1 concludes
  otherwise, this is the wrong skill (see Goal).

## Execution Steps

### 1. Resolve the target and confirm this is genuinely an upgrade

Parse `$ARGUMENTS` for an `FR-XXX` ID and a description of how its behavior is changing. Locate
the owning `specs/<slug>/spec.md` and read the FR's current text and Acceptance Scenario(s), plus
its existing evidence in `sbr/rtm/<slug>.rtm.md` if a row exists. If the description in
`$ARGUMENTS` is ambiguous about what's actually changing (old behavior vs. new intended
behavior), ask before editing anything.

If the target names behavior with **no existing FR** to amend, or an apparent defect nobody
deliberately decided to change, STOP per the Goal section above rather than proceeding under the
wrong process.

Read `sbr/README.md`'s adapter section for the tier→directory→run-command mapping before
continuing.

### 2. Update the spec and design first (mandatory)

Amend the `FR-XXX` text and its Acceptance Scenario(s) in `spec.md` to describe the new intended
behavior — nothing else in that file. State the exact diff (old FR text, new FR text) in the
report. If `plan.md`, `data-model.md`, `contracts/`, or similar design artifacts for the owning
feature describe the behavior that's changing (a changed field, a changed algorithm, a changed
contract shape), update those too, scoped to exactly what changed. If the change has no footprint
beyond `spec.md` (a pure logic change with no design-doc-level description), say so explicitly
rather than skipping this step silently.

### 3. Update or write tests for the new behavior; confirm they fail against the old code

At the tier(s) appropriate to the FR (per the adapter), update any existing test whose assertions
encode the *old* behavior to assert the *new* behavior instead, and/or add new tests for behavior
the old suite never exercised. Run them via the adapter's run command against the **current,
pre-upgrade** implementation and confirm they fail for the right reason. Record, for every
modified test, the exact assertion diff (old → new).

If a test unexpectedly passes against the old code, stop and reconcile before continuing: either
Step 1's "old behavior" premise was wrong (re-check it), or the test doesn't actually exercise the
changing logic yet (rewrite it). Never proceed with a test that isn't demonstrably red against the
pre-upgrade implementation — that red is this skill's whole basis for trusting the test, in place
of the posthoc genuineness check extension/bugfix work relies on.

**SBR-Trace annotation, every test updated or added in this step.** Add or update the
`// SBR-Trace: FR-XXX — <behavior>` comment directly above the test function (see
`sbr/README.md` → "SBR-Trace test annotations" and the adapter for this project's exact Go
format), describing the *new* behavior — the annotation always tracks current intent, never the
superseded one. This is also the point to fix any sibling test's stale annotation your Step 1
reading turned up.

**Sibling check.** While confirming red against the old code, also run the tier's sibling tests
in the same file (and, where annotations exist, any other test elsewhere annotated with this
`FR-XXX`) — not just the test(s) you're deliberately updating. See `sbr/README.md` → "Mutation
blast radius (extending Step D)" for the same logic applied here: a sibling that also encodes the
old behavior but wasn't included in this step's update scope is a completeness gap in Step 3, not
a finding to defer — fold it into this step's update now, since it's the same change already
underway, not a new one. A sibling that stays green and unaffected is the expected case for
anything genuinely unrelated to the changing behavior.

### 4. Implement the change

Implement the minimum change to the implementation logic that makes the updated/new tests pass,
scoped to exactly what Step 2's updated spec/design describes. Do not touch any test from Step 3
to make it pass — if a test itself turns out to be wrong, stop and report it rather than adjusting
it unilaterally.

### 5. Verify

Run the updated/new tests again and confirm green. Run the full existing suite for every touched
tier (prefer the adapter's combined command when the change touches shared code) and confirm
nothing else newly fails. Any such failure is an unintended regression — a caller or scenario
this upgrade wasn't meant to affect — and must be resolved (by adjusting the implementation, not
by touching an unrelated test) before proceeding.

### 6. Review for affected callers and scenarios

Ask, and answer explicitly in the report:

- What other callers, adjacent scenarios in the same feature/RTM, or duplicated FRs in sibling
  features (the same cross-RTM duplication pattern `sbr/README.md`'s worked examples already
  show, e.g. FR-011/FR-012 in `003-organizations-administration.rtm.md`) assume the *old*
  behavior?
- Could any of those still be passing not because they're unaffected, but because their
  assertions happen not to exercise the specific thing that changed? Distinguish "genuinely
  unaffected" from "coincidentally still green" before concluding a caller is fine.

List findings as follow-up candidates (file:line, what to check) rather than automatically
expanding this run's scope — fold in a fix only if it's trivially the same one-line change applied
to a true sibling, and say explicitly that this happened, mirroring `sbr-bugfix` Step 8's
discipline.

### 7. Update traceability

Update the feature RTM row for the changed `FR-XXX` in `sbr/rtm/<feature-slug>.rtm.md`: refresh
its test-evidence citations to the updated/new tests, keep (or set) Boundary Status `Complete`,
and add a Notes entry recording this as a **behavior change** — old requirement summary → new,
with today's date — distinguishing it from an ordinary gap-closure so a later reader knows the
row's history includes an intentional requirement change, not just a coverage fix. Never write to
a `bugfix-*.rtm.md` file — a feature upgrade is never routed there.

### 8. Report

Print a concise summary: the `FR-XXX` changed, old behavior → new behavior, spec/design files
touched with their diffs, test(s) updated/added (with the old→new assertion diff for modified
ones, their SBR-Trace annotations, and confirmation each was red against the pre-upgrade
implementation), Step 3's sibling-check findings (any additional test folded into this step's
scope), the implementation change made, full-suite verification result, Step 6's affected-caller
review findings (resolved-in-this-pass vs. flagged-for-follow-up), and the RTM row updated.

## Post-Execution Checks

After producing the result, check if `.specify/extensions.yml` exists in the project root.

- If it exists, read it and look for entries under the `hooks.after_sbr_feature_upgrade` key
- If the YAML cannot be parsed or is invalid, skip hook checking silently and continue normally
- Filter out hooks where `enabled` is explicitly `false`. Treat hooks without an `enabled` field as enabled by default.
- For each remaining hook, do **not** attempt to interpret or evaluate hook `condition` expressions:
  - If the hook has no `condition` field, or it is null/empty, treat the hook as executable
  - If the hook defines a non-empty `condition`, skip the hook and leave condition evaluation to the HookExecutor implementation
- Report the upgrade outcome (FR changed, tests updated, suite verification result) before
  listing any hooks, so users can decide whether to run optional follow-up commands (e.g.
  re-running `sbr-audit` to confirm the RTM reflects the change project-wide).
- When constructing slash commands from hook command names, replace dots (`.`) with hyphens (`-`).
- For each executable hook, output the same optional/mandatory blocks used in Pre-Execution
  Checks, substituting the after-hook framing, and actually invoke mandatory hooks before
  finishing.
- If no hooks are registered or `.specify/extensions.yml` does not exist, skip silently
