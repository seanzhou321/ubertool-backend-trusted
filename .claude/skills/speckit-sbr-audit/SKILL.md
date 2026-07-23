---
name: "speckit-sbr-audit"
description: "Audit a feature spec's functional requirements against the project's test tiers and write/update its SBR Requirements Traceability Matrix (RTM)."
argument-hint: "<feature-slug|all> (optional — defaults to the current ambient feature)"
compatibility: "Requires spec-kit project structure with .specify/ directory and an sbr/README.md adapter"
metadata:
  author: "sbr-extension"
  source: "sbr/README.md"
user-invocable: true
disable-model-invocation: false
---


## User Input

```text
$ARGUMENTS
```

You **MUST** consider the user input before proceeding (if not empty).

## Pre-Execution Checks

**Check for extension hooks (before audit)**:

- Check if `.specify/extensions.yml` exists in the project root.
- If it exists, read it and look for entries under the `hooks.before_sbr_audit` key
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

For one or more feature specs, read every `FR-XXX` functional requirement in `spec.md`,
search the project's test tiers for evidence that each requirement is actually verified, and
write or update `sbr/rtm/<feature-slug>.rtm.md` per the schema in `sbr/README.md`. This
surfaces requirements that are documented but not verified anywhere — a gap that code
coverage and a passing build cannot reveal. Read `sbr/README.md` in full before the first
execution step; it defines the schema, the boundary-status vocabulary, and (in its "This
repo's adapter" section) the concrete tier→directory mapping and search conventions this
skill must use for **this** project. A ported copy of this skill in another project should
only ever need that project's own `sbr/README.md` adapter section to behave correctly — the
steps below never hardcode a tier's directory name; they always say "per the adapter."

This is an **audit**, not a gate: it runs against already-written specs and already-written
code (retrofit mode, see `sbr/README.md` → "Audit mode vs. gating mode"). It does not block
or require anything before it can run.

## Operating Constraints

**STRICTLY READ-ONLY against `spec.md` and everything under the test directories named in the
adapter mapping.** This command MUST NOT:

- modify `spec.md`, `plan.md`, or any other spec-kit artifact;
- modify, create, or delete any test file, or add trace comments to test code — a trace-comment
  convention is a legitimate future improvement (see `sbr/README.md`) but is a change to the
  authoritative test suite and requires its own explicit review, never done as a side effect of
  an audit;
- modify any application code.

Its **only** writes are creating or fully rewriting `sbr/rtm/<feature-slug>.rtm.md` file(s) —
one per audited feature. Rewriting (not appending) is correct here, unlike `speckit-converge`'s
append-only `tasks.md` contract: an RTM is a current-state snapshot of traceability, not a
history log, so a re-run must reflect the present state of spec.md and the test suite exactly,
including rows that have changed status since the last run.

**Never fabricate a trace.** A cell in the RTM must cite a concrete, real function name (and
subtest name where applicable) that you have actually located via search — never a plausible
guess. When search doesn't turn up confident evidence, the correct entry is an explicit gap or
`Unclassified`, not an invented citation.

**Constitution Authority**: if a mapped test tier or directory named in the adapter section no
longer matches the project's constitution (`.specify/memory/constitution.md`), stop and report
the mismatch rather than silently searching the wrong place.

## Execution Steps

### 1. Load the adapter

Read `sbr/README.md` in full. Extract the "This repo's adapter" section's tier→directory
mapping table and search conventions. If `sbr/README.md` does not exist, STOP and report that
this skill requires the SBR adapter doc to be created first — it has no built-in default
mapping to fall back on (a hardcoded default would silently defeat the portability the adapter
split exists to provide).

### 2. Resolve target spec(s)

This skill's feature resolution deliberately differs from other `speckit-*` skills: RTM
auditing is inherently cross-feature and retrospective (every feature's spec already coexists
in `specs/` regardless of which branch is checked out), so it does not rely solely on the
ambient "current feature" the way `speckit-plan`/`speckit-tasks` do.

- If `$ARGUMENTS` is `all`: target every `specs/*/spec.md`.
- If `$ARGUMENTS` names a slug or slug-prefix (e.g. `003` or
  `003-organizations-administration`): resolve via a glob against `specs/<arg>*/spec.md`. If
  it matches zero or more than one directory, STOP and report the ambiguity/absence rather
  than guessing.
- If `$ARGUMENTS` is empty: fall back to the ambient current feature, exactly like other
  `speckit-*` skills — run `.specify/scripts/powershell/check-prerequisites.ps1 -PathsOnly
  -Json` (the `-PathsOnly` form does pure path resolution with no `plan.md`/`tasks.md`
  validation, since this audit only needs `spec.md`) and use its `FEATURE_DIR`.

For each resolved feature directory, continue with steps 3-6 independently.

### 3. Extract functional requirements

Read `FEATURE_DIR/spec.md`. Extract every `**FR-XXX**: ...` entry verbatim from the
`### Functional Requirements` section, preserving its ID and full requirement text.

### 4. Search for test evidence, per tier defined in the adapter

For each FR, and for each tier in the adapter's mapping table (in this project: L1 Unit → `tests/unit`,
L2 Integration → `tests/integration`, L3 E2E → `tests/e2e`, Grounding → `tests/smoke` +
`tests/e2e` journeys — always read the actual current adapter table from Step 1 rather than
assuming this list stays correct):

- Derive search terms from the FR's prose: the RPC/method name(s), entity name(s), and
  distinguishing behavior keywords it names.
- Search test **function names** first (per the adapter's naming convention for this
  language/repo).
- For any tier the adapter flags as using sub-grouped tests (in this project: e2e's
  `t.Run("...")` subtests under one broad `Test<Service>_E2E` function) — search subtest
  string literals too, not just top-level function names. Do not rely on top-level names alone
  for such tiers; that under-reports coverage.
- Only read a full test body when a name match is ambiguous enough that the citation would
  otherwise be a guess.
- Record, per tier: either a concrete citation (`function_name (file:line)`, or
  `TestX_E2E > "subtest name" (file:line)` for subtest evidence), or that no plausible match
  was found.

### 5. Classify each FR's boundary status

Using the vocabulary defined in `sbr/README.md`:

- **`Complete`** if the requirement has citable evidence at the tier(s) appropriate to its
  nature. Not every FR needs every tier populated — judge per-requirement (e.g. a pure
  data-shape requirement may only need L1 + L3 evidence) and record the reasoning in Notes.
- **`Gap — <tier>`** naming every tier where evidence should reasonably exist but does not.
- **`Unclassified`** where confidence is too low to call it either way — never force a
  guess into `Complete`.

### 6. Write the RTM

Create or fully rewrite `sbr/rtm/<feature-slug>.rtm.md` per the schema in `sbr/README.md`:
header (source spec path, pointer to the adapter section in effect, generation date, mode =
`retrofit audit`), the FR table with one column per tier plus Boundary Status and Notes, and a
footer summary (total FR count, gap count, explicit list of FR-IDs with gaps).

### 7. Report

After processing all resolved features, print a concise console summary: features audited,
total FRs checked, total gaps found (broken down by tier), and the path(s) to the RTM file(s)
written or updated. If any FR could not be classified with confidence, list those explicitly
as needing human review rather than folding them silently into the gap count.

## Post-Execution Checks

After producing the result, check if `.specify/extensions.yml` exists in the project root.

- If it exists, read it and look for entries under the `hooks.after_sbr_audit` key
- If the YAML cannot be parsed or is invalid, skip hook checking silently and continue normally
- Filter out hooks where `enabled` is explicitly `false`. Treat hooks without an `enabled` field as enabled by default.
- For each remaining hook, do **not** attempt to interpret or evaluate hook `condition` expressions:
  - If the hook has no `condition` field, or it is null/empty, treat the hook as executable
  - If the hook defines a non-empty `condition`, skip the hook and leave condition evaluation to the HookExecutor implementation
- Report the audit outcome (features audited, gap count) before listing any hooks, so users
  can decide whether to run optional follow-up commands.
- When constructing slash commands from hook command names, replace dots (`.`) with hyphens (`-`).
- For each executable hook, output the same optional/mandatory blocks used in Pre-Execution
  Checks, substituting the after-hook framing, and actually invoke mandatory hooks before
  finishing.
- If no hooks are registered or `.specify/extensions.yml` does not exist, skip silently
