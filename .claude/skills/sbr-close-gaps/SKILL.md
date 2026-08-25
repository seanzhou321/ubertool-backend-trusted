---
name: "sbr-close-gaps"
description: "Batch-close every Gap/Unclassified (or Vulnerable) row across one or more RTMs by dispatching sbr-bugfix once per row, sequentially, and reporting a consolidated result."
argument-hint: "<feature-slug|all|security> (optional — defaults to whatever sbr-audit most recently reported in this session)"
compatibility: "Requires spec-kit project structure with sbr/README.md's adapter and the sbr-audit / sbr-bugfix skills installed"
metadata:
  author: "sbr-extension"
  source: "sbr/README.md — batch driver for closing sbr-audit findings via sbr-bugfix"
user-invocable: true
disable-model-invocation: false
---


## User Input

```text
$ARGUMENTS
```

You **MUST** consider the user input before proceeding (if not empty).

## Pre-Execution Checks

**Check for extension hooks (before batch close)**:

- Check if `.specify/extensions.yml` exists in the project root.
- If it exists, read it and look for entries under the `hooks.before_sbr_close_gaps` key
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
    ```
    After emitting the block above you MUST actually invoke the hook and wait for it to finish before continuing.

- If no hooks are registered or `.specify/extensions.yml` does not exist, skip silently

## Goal

Given one or more RTM files, find every row currently classified `Gap-*`, `Unclassified`, or
(security RTM only) `Vulnerable`, and close each one by invoking `/sbr-bugfix` with that
row's `FR-ID`/`AV-ID` as its argument — one row at a time, waiting for each run to finish before
starting the next. This skill is pure orchestration: it contains no diagnostic or test-writing
logic of its own. All of that lives in `sbr-bugfix`, invoked fresh per row so each fix
gets its own independent root-cause analysis, undiluted by batch context from prior rows.

This is the "actually implement the missing tests, at scale" counterpart to
`sbr-audit` (which only ever reads and reports) — see `sbr/README.md` for the shared
vocabulary and the "Posthoc test-genuineness check" section that every dispatched
`sbr-bugfix` run applies to the tests it writes.

## Operating Constraints

- **Never invent the work list.** Read it directly from the RTM file(s) resolved in Step 1, as
  currently written. Nothing here re-derives gaps from spec/test search — that is
  `sbr-audit`'s job, run separately (or via its own `after_implement` hook) to produce
  the RTM this skill then consumes.
- **Strictly sequential, never parallel.** Shared repo and test-suite state, and
  `sbr-bugfix`'s own contract already assumes an uncontested working tree during its
  verification step.
- **Re-check each row immediately before dispatching it.** A prior row's fix may have already
  closed a later row as a hidden-problem follow-up (`sbr-bugfix` Step 7 explicitly
  allows folding in a trivially-identical sibling fix). If the row is no longer `Gap`/
  `Unclassified`/`Vulnerable` when its turn comes, skip it with a one-line note instead of
  re-running a fix that already landed.
- **If any dispatched `sbr-bugfix` run stops to ask the user something** (ambiguous
  target, an existing-test conflict, an unresolvable target) rather than completing, halt the
  batch at that row. Do not skip it and continue — report which rows closed, which one blocked
  and why, and which remain unattempted.
- **Cap unattended scope.** If the work list exceeds 10 rows, stop before starting and ask for
  confirmation, naming the count and the RTM(s) involved, rather than silently running a long
  unattended chain of code-writing operations.
- **Never rewrite RTM structure directly.** Each dispatched `sbr-bugfix` call updates its
  own row per its own Step 8 traceability rules; this skill only reads RTMs to build and refresh
  the work list.

## Execution Steps

### 1. Resolve target RTM(s)

Parse `$ARGUMENTS`:

- A feature slug or slug-prefix (e.g. `003`): resolve via glob against `sbr/rtm/<arg>*.rtm.md`
  (excluding `bugfix-*.rtm.md`, per Step 2). Zero or multiple matches: STOP and report the
  ambiguity/absence.
- `all`: every `sbr/rtm/*.rtm.md` except `sbr/rtm/bugfix-*.rtm.md` (a closed-defect log with no
  `Gap` rows by construction — nothing to close there).
- `security`: the security RTM named in the adapter's "Security audit inputs" section.
- Empty: use whatever `sbr-audit` most recently reported in this same session. If there
  is no such prior report, ask the user which RTM(s) to close gaps against rather than guessing.

### 2. Build the work list

Read the resolved RTM(s). Extract every row whose Boundary Status is `Gap-*`, `Unclassified`, or
(security RTM only) `Vulnerable`, preserving file order (and file order across multiple RTMs when
`all` was resolved). If the list is empty, report "no gaps found in the resolved RTM(s)" and
stop — there is nothing further to do.

### 3. Confirm scope if large

If the work list has more than 10 rows, stop and ask the user to confirm before proceeding,
stating the count and which RTM(s) they come from (Operating Constraints).

### 4. Dispatch sequentially

For each row, in order:

1. Re-read its current Boundary Status directly from the RTM file. If it is no longer `Gap-*`/
   `Unclassified`/`Vulnerable`, record it as **skipped (already resolved)** and move to the next
   row without dispatching anything.
2. Otherwise, invoke `/sbr-bugfix <FR-ID or AV-ID>` and wait for it to finish.
3. Record the outcome: **closed** (with the traceability home it updated), or **blocked** (the
   run stopped to ask the user something instead of completing).
4. If the outcome was **blocked**, stop iterating entirely — do not attempt any remaining rows —
   and proceed to Report.

### 5. Report

Print a table: row ID, outcome (closed / skipped-already-resolved / blocked / not attempted),
and traceability home updated where applicable. Summarize totals (closed, skipped, blocked,
remaining). If a row blocked the batch, state which one and why, and that the remaining rows were
not attempted as a result.

## Post-Execution Checks

After producing the result, check if `.specify/extensions.yml` exists in the project root.

- If it exists, read it and look for entries under the `hooks.after_sbr_close_gaps` key
- If the YAML cannot be parsed or is invalid, skip hook checking silently and continue normally
- Filter out hooks where `enabled` is explicitly `false`. Treat hooks without an `enabled` field as enabled by default.
- For each remaining hook, do **not** attempt to interpret or evaluate hook `condition` expressions:
  - If the hook has no `condition` field, or it is null/empty, treat the hook as executable
  - If the hook defines a non-empty `condition`, skip the hook and leave condition evaluation to the HookExecutor implementation
- Report the batch outcome (closed/skipped/blocked/remaining counts) before listing any hooks, so
  users can decide whether to run optional follow-up commands (e.g. re-running
  `sbr-audit` to confirm the RTM now reflects every closure).
- When constructing slash commands from hook command names, replace dots (`.`) with hyphens (`-`).
- For each executable hook, output the same optional/mandatory blocks used in Pre-Execution
  Checks, substituting the after-hook framing, and actually invoke mandatory hooks before
  finishing.
- If no hooks are registered or `.specify/extensions.yml` does not exist, skip silently
