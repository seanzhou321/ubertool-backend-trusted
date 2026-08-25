---
name: "sbr-security-audit"
description: "Run an adversarial, endpoint-by-endpoint security audit of the API surface against OWASP API Security Top 10 categories and write/update the project's unified security RTM."
argument-hint: "<service-slug|endpoint-name|all> (optional — defaults to 'all')"
compatibility: "Requires spec-kit project structure with sbr/README.md's adapter, including its 'Security audit inputs' subsection"
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
- If it exists, read it and look for entries under the `hooks.before_security_audit` key
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

For every endpoint in the project's API surface (or a named subset), think like an attacker:
enumerate concrete, code-grounded ways a hostile caller could abuse it, check whether the
codebase actually defends against each, and write or update the project's unified security RTM
per the "Adversarial (security) audit mode" schema defined in `sbr/README.md`. Read
`sbr/README.md` in full before the first execution step — its Portable section defines the
extended boundary-status vocabulary (`Vulnerable`, `Accepted Risk`, on top of the base
`Complete`/`Gap`/`Unclassified`) this skill must use, and its adapter's "Security audit inputs"
subsection names the concrete files (endpoint inventory, auth interceptor, security-level
config, rate limiter) this skill must read rather than rediscover. A ported copy of this skill
in another project should only ever need that project's own adapter section (rewritten per its
own stack) to behave correctly — never hardcode a file path below that the adapter already
names.

This is an **audit**, not a penetration test and not an auto-fixer: it never sends traffic to a
running instance, and it never modifies application code — even when it finds a live
vulnerability, its job is to document the defect precisely enough that fixing it is a five-minute
follow-up, not to make the fix itself.

## Operating Constraints

**STRICTLY READ-ONLY against everything except the security RTM file.** This command MUST NOT:

- modify any proto file, handler, service, or repository code, even to fix a vulnerability it
  finds;
- modify, create, or delete any test file;
- modify `spec.md`, `plan.md`, or any other spec-kit artifact, or any other RTM file under
  `sbr/rtm/` besides the one named in the adapter's "Security audit inputs".

Its **only** write is creating or updating the security RTM file named in the adapter (this
project: `sbr/rtm/009-security.rtm.md`). Unlike `sbr-audit`'s per-feature RTMs (one file,
fully rewritten per feature), this file is a single document with one section per service — a
scoped run (a specific service or endpoint) rewrites only that section and the header/footer
summary counts; only an `all` run rewrites the whole file end to end.

**Never fabricate a vulnerability, and never fabricate a trace.** A `Vulnerable` classification
requires having actually read the handler → service → repository code path that would need to
defend against the attack, and being able to say what specific check is missing — ideally by
name, contrasting with a sibling endpoint elsewhere in the same service that performs the check
correctly (the single strongest evidence something is a bug rather than a deliberate design
choice). A test-evidence citation must be a concrete, real function name (and subtest name where
applicable) actually located via search — never a plausible guess. When code inspection is
inconclusive, the correct classification is `Unclassified`, never a confident-looking guess in
either direction.

**High-severity findings get a second read before they're written down.** Before finalizing any
`Vulnerable` classification tagged Critical or High, re-read the exact lines being cited one more
time in isolation (not just trusting the first pass) — this catches the difference between "I
didn't find a check" (weak evidence, could be a search miss) and "I confirmed there is no check"
(strong evidence).

## Execution Steps

### 1. Load the adapter

Read `sbr/README.md` in full, including the "Adversarial (security) audit mode" subsection of
the Portable section and the "Security audit inputs" subsection of the adapter. If either is
missing, STOP and report that this skill requires both before it can run — it has no built-in
default vocabulary or file-location mapping to fall back on.

### 2. Establish the auth/authz baseline once

Before touching individual endpoints, read the auth-enforcement point, token issuance/validation,
and rate-limiting files named in "Security audit inputs" (in this project: the auth interceptor,
`internal/security/token.go`, and the rate-limit interceptor/limiter). This is cross-cutting
infrastructure shared by every endpoint — establishing it once avoids re-deriving the same facts
per endpoint and gives every per-endpoint row a consistent baseline to cite (e.g. "identity comes
from the interceptor-injected context, not a client-suppliable field" applies identically to
dozens of endpoints and should be stated once, not 40 times). Note anything cross-cutting that
deviates from what the adapter describes as of its last update — the rate-limiting scope in
particular is worth re-verifying every run, since it's the kind of thing that silently regresses.

### 3. Resolve scope

- If `$ARGUMENTS` is empty or `all`: audit every endpoint in the inventory named by the adapter
  (every `rpc` across every proto file, plus every non-gRPC route).
- If `$ARGUMENTS` names a service (e.g. `RentalService` or `rental`) or a single endpoint (e.g.
  `CompleteRental`): resolve it against the proto inventory. If it matches zero or more than one
  service/endpoint ambiguously, STOP and report the ambiguity rather than guessing.

### 4. Enumerate endpoints in scope

For each proto file in scope, extract every `rpc Name(Request) returns (Response)` along with
its configured security level from the endpoint-security-config file named in the adapter
(explicit entry, or "relies on fail-closed default" if absent from the map — the latter is
itself worth a row: an unmapped endpoint is a maintainability gap even when the runtime default
is safe). Include non-gRPC routes named in the adapter's endpoint inventory entry.

### 5. Per-endpoint adversarial analysis

For each endpoint, working from the handler down through the service layer to the repository
layer (not from requirement prose — there may be none):

- **Identify the authorization logic actually present**: cite file:line for any
  ownership/membership/role check gating the specific resource the request targets. If none
  exists where one plausibly should, that is the seed of a finding, not a document-and-move-on
  item.
- **Enumerate attack vectors against the OWASP API Security Top 10 categories** named in the
  adapter, applying only the categories that plausibly apply to what this endpoint does (a
  static-lookup endpoint with no resource ID has no BOLA surface; a read-only endpoint has no
  mass-assignment surface). Ground every vector in the endpoint's actual request/response fields
  and code paths — never generic boilerplate ("check for SQL injection" without having looked at
  whether the query is parameterized).
- **Search for existing test evidence** per the tier and search conventions in the adapter
  (including the "e2e tests are a trap" subtest-string-literal caveat) for each attack vector
  identified. Cite `TestX > "subtest name" (file:line)` format when found.
- **Classify** using the extended vocabulary from `sbr/README.md`: `Complete`, `Gap — <tier>`,
  `Vulnerable (<severity>)`, `Accepted Risk`, or `Unclassified`.
- **For `Vulnerable` findings**, write a suggested regression test (name + one-line shape) that
  would fail against current code and should pass once the fix lands — this is what makes the
  finding actionable rather than just alarming.

### 6. Parallelize for full-surface runs

An `all` run across every service in a nontrivial API is too much ground to cover in a single
context without either running out of budget or skimming. For an `all` (or multi-service) scope,
partition the services into 2-4 roughly-equal-effort groups (e.g. by domain: identity/access,
assets/transactions, financial/notifications) and dispatch one research agent per group in
parallel, each briefed with: the auth/authz baseline from Step 2, the relevant proto/handler/
service/repository files for its group, the OWASP category checklist, and the search conventions
from the adapter. Each agent should return per-endpoint findings in the Step 5 shape (endpoint,
authorization logic found, attack vectors, test evidence, classification) rather than prose
summary, so the results merge cleanly into one RTM. Synthesize their reports yourself — do not
paste agent output verbatim into the RTM without applying the Step 5 classification discipline
and the high-severity re-read requirement above, since agents may not weight severity consistently
with each other. For a single-service or single-endpoint scope, do Step 5 directly without
spawning agents — the overhead isn't justified.

### 7. Write the RTM

Create or update the security RTM file named in the adapter, per the schema established in
`sbr/README.md` and (for this project) demonstrated in `sbr/rtm/009-security.rtm.md`: a header
(scope of this run, adapter pointer, generation/update date, mode = `adversarial audit`), an
Executive Summary of findings ranked by severity, one section per service (plus a Global /
Cross-Cutting section for interceptor/rate-limit/config-completeness findings, and a section for
any non-gRPC routes), each with a table of AV-ID | Endpoint | Attack Vector | test-tier evidence
columns | Boundary Status | Notes, and a footer Summary (total attack vectors, count per boundary
status, explicit list of `Vulnerable` AV-IDs in priority order). For a scoped run, only the
section(s) covering the resolved scope are rewritten — regenerate the footer Summary counts
across the *whole* file (not just the rewritten section) so they stay accurate, and preserve
every other section's content and AV-ID numbering exactly as found.

### 8. Report

Print a concise console summary: scope audited, endpoint count, attack-vector count, and a
breakdown by boundary status — call out the `Vulnerable` count and its highest-severity entries
explicitly rather than folding them into a single aggregate number, since these are the ones that
need a human decision before the next release. Include the path to the RTM file written or
updated.

## Post-Execution Checks

After producing the result, check if `.specify/extensions.yml` exists in the project root.

- If it exists, read it and look for entries under the `hooks.after_security_audit` key
- If the YAML cannot be parsed or is invalid, skip hook checking silently and continue normally
- Filter out hooks where `enabled` is explicitly `false`. Treat hooks without an `enabled` field as enabled by default.
- For each remaining hook, do **not** attempt to interpret or evaluate hook `condition` expressions:
  - If the hook has no `condition` field, or it is null/empty, treat the hook as executable
  - If the hook defines a non-empty `condition`, skip the hook and leave condition evaluation to the HookExecutor implementation
- Report the audit outcome (scope, attack-vector count, `Vulnerable` count) before listing any
  hooks, so users can decide whether to run optional follow-up commands (e.g. dispatching fixes
  for the highest-severity `Vulnerable` findings).
- When constructing slash commands from hook command names, replace dots (`.`) with hyphens (`-`).
- For each executable hook, output the same optional/mandatory blocks used in Pre-Execution
  Checks, substituting the after-hook framing, and actually invoke mandatory hooks before
  finishing.
- If no hooks are registered or `.specify/extensions.yml` does not exist, skip silently
