# Feature Specification: Tools & Image Storage (As-Built)

**Feature Branch**: `007-tools-image-storage`

**Created**: 2026-07-22

**Status**: Draft — **Known Discrepancy 1 (missing Tool ownership checks) was found
CRITICAL and fixed the same session it was discovered, 2026-07-22.**

**Input**: Retrofit specification for the existing, already-implemented and deployed Tools
and Image Storage features. Per project constitution Principle I ("Code Is Truth"), this
document describes verified current behavior of `internal/service/tool.go`,
`internal/service/image_storage.go`, `internal/api/grpc/tool.go`,
`internal/api/grpc/image_storage.go`, the `tools`/`tool_images` tables in
`podman/trusted-group/postgres/ubertool_schema_trusted.sql`, and
`api/proto/ubertool_trusted_backend/v1/tool_service.proto` +
`image_storage_service.proto` — cross-checked against
`docs/design/grpc_api_business_logic.md`'s "Tools" and "Image Storage" sections and
`docs/design/image-storage/`. It is **not** a proposal for new behavior; every gap found is
called out in "Known Discrepancies" below.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Tool Listing Ownership & Lifecycle (Priority: P1)

An owner creates, views, updates, and removes their own tool listings.

**Why this priority**: The foundational data every other capability in this domain and in
Rentals operates on. This story also carries the session's second critical finding.

**Independent Test**: Create a tool as user A; confirm user B cannot update or delete it;
confirm user A can.

**Acceptance Scenarios**:

1. **Given** an authenticated user, **When** `AddTool` is called, **Then** a `tools` row is
   created with `owner_id` from the JWT, `status = AVAILABLE`, and the request's `duration`
   persisted as `duration_unit`; any provided image URLs are inserted as `tool_images` rows
   (`file_name`/`file_path`/`thumbnail_path` all set to the raw URL — see Known
   Discrepancy 3).
2. **Given** a `tool_id`, **When** `GetTool` is called, **Then** the tool and its images are
   returned, with owner details populated including only the organizations shared between
   the tool's owner and the requesting caller.
3. **Given** the caller is the tool's owner, **When** `UpdateTool` is called, **Then** the
   mutable fields (name, description, categories, prices, replacement cost, condition,
   metro, duration) are overwritten. **As of the 2026-07-22 fix** (Known Discrepancy 1),
   this now requires the caller's ID to match `tool.OwnerID`, fetched fresh from the
   database before the write.
4. **Given** the caller is **not** the tool's owner, **When** `UpdateTool` or `DeleteTool`
   is called, **Then** the call is rejected ("unauthorized: only the tool owner may
   update/delete this tool") — enforced as of the same fix.
5. **Given** the caller is the tool's owner, **When** `DeleteTool` is called, **Then** the
   tool is soft-deleted (`deleted_on` timestamp set, row otherwise retained).
6. **Given** an authenticated user, **When** `ListMyTools` is called, **Then** only tools
   they own are returned, paginated.

---

### User Story 2 - Tool Discovery (Priority: P1)

Org members browse and search for tools available in their community's metro area.

**Why this priority**: The primary way renters find something to rent — equally
foundational to User Story 1 for the app's core loop.

**Independent Test**: As a member of an org in "San Jose", call `SearchTools` with a query
and confirm only tools whose owner shares an organization with the caller are returned,
even if other tools match the text/metro filter.

**Acceptance Scenarios**:

1. **Given** an `organization_id`, **When** `ListTools`/`SearchTools` resolves the search
   metro, **Then** it is taken from that organization's `metro` field rather than a
   caller-supplied value; if no `organization_id` is given, `metro` must be supplied
   directly or the call is rejected.
2. **Given** a non-empty `query`, **When** `SearchTools` is called, **Then** results are
   filtered by the resolved metro, `query` text, optional `categories`/`max_price`, and
   `condition` (defaulting to `NOT_DAMAGED` when unspecified).
3. **Given** search results from the repository, **When** they are post-processed,
   **Then** each result's owner is populated with only the organizations shared between
   that owner and the caller, and any tool whose owner shares **no** organization with the
   caller is filtered out of the final response entirely — even though the repository-level
   query already matched it.
4. **Given** an empty `query`, **When** `SearchTools` is called, **Then** it is rejected
   ("query parameter is required and cannot be empty").
5. **Given** no filters, **When** `ListToolCategories` is called, **Then** a static,
   hardcoded category list is returned (not queried from `DISTINCT categories` in the
   `tools` table as `grpc_api_business_logic.md` describes — see Known Discrepancy 4).

---

### User Story 3 - Image Upload Pipeline (Priority: P2)

An owner requests a presigned upload URL, uploads directly to cloud storage, then confirms
the upload so the backend can validate it and generate a thumbnail.

**Why this priority**: A three-step, owner-only flow that's independent of the read-heavy
discovery/visibility stories.

**Independent Test**: Call `GetUploadUrl` for a tool you own; upload bytes to the returned
URL (or seed the storage backend directly in tests); call `ConfirmImageUpload` and confirm
the image transitions to `CONFIRMED` and a background goroutine eventually populates
`thumbnail_path`.

**Acceptance Scenarios**:

1. **Given** the caller owns `tool_id` (or `tool_id = 0` for a not-yet-created tool),
   **When** `GetUploadUrl` is called, **Then** a `PENDING` `tool_images` row is created, a
   15-minute presigned PUT URL and a 1-hour presigned GET URL are generated, and the
   storage path embeds the generated image ID.
2. **Given** the caller does not own the tool, **When** `GetUploadUrl` is called, **Then**
   it is rejected ("unauthorized: you do not own this tool").
3. **Given** a `PENDING` image the caller owns (`tool_images.user_id = caller`), **When**
   `ConfirmImageUpload` is called, **Then** the service verifies the file actually exists
   in storage (`FileExists`), marks the record `CONFIRMED`, sets it as the tool's primary
   image if it's the tool's first, and launches a **background goroutine** that decodes
   the image, scales it to fit 300×300 (preserving aspect ratio, BiLinear), re-encodes as
   85%-quality JPEG, and updates `thumbnail_path` — all after the RPC has already returned
   to the caller.
4. **Given** the image is not owned by the caller, or is not currently `PENDING`, or the
   file does not exist in storage, **When** `ConfirmImageUpload` is called, **Then** it is
   rejected accordingly.
5. **Given** an image belonging to a tool, **When** `GetDownloadUrl` is called, **Then**
   access is granted if the caller owns the tool **or** the tool's status is `AVAILABLE`
   (treated as public); a 1-hour presigned GET URL is returned for the thumbnail (falling
   back to the full image if no thumbnail exists yet) or the full image.

---

### User Story 4 - Image Management (Priority: P2)

An owner deletes an image or changes which image is primary for their tool; any caller can
list a tool's confirmed images.

**Why this priority**: Secondary curation actions on top of the upload pipeline.

**Independent Test**: Upload two confirmed images for a tool; call `SetPrimaryImage` to
promote the second; confirm the first is no longer primary and the unique-primary-per-tool
database constraint is never violated (the repository does the swap inside a transaction).

**Acceptance Scenarios**:

1. **Given** a `tool_id`, **When** `GetToolImages` is called, **Then** all `CONFIRMED`
   images for that tool are returned, ordered primary-first. **As-built**, this performs no
   access check of any kind — any authenticated caller can list any tool's confirmed
   images regardless of ownership or the tool's status (see Known Discrepancy 2).
2. **Given** the caller owns the tool, **When** `DeleteImage` is called, **Then** the
   image's files are best-effort deleted from cloud storage (a storage-delete failure is
   logged, not fatal), the `tool_images` row is soft-deleted, and if the deleted image was
   primary, the oldest remaining confirmed image is promoted to primary.
3. **Given** the caller does not own the tool, **When** `DeleteImage` is called, **Then**
   it is rejected.
4. **Given** the caller owns the tool and the target image is `CONFIRMED` and belongs to
   that tool, **When** `SetPrimaryImage` is called, **Then** the previous primary (if any)
   is unset and the target is set primary, inside a single database transaction (enforcing
   the schema's `idx_tool_images_primary_unique` constraint is never violated even
   transiently).

---

### Edge Cases

- `AddTool`'s image-URL ingestion path stores the same raw URL string in `file_name`,
  `file_path`, and `thumbnail_path` — this is a different, simpler code path than the
  presigned-upload pipeline (User Story 3) and does not go through image validation,
  ownership double-checks, or thumbnail generation.
- `SearchTools`'s shared-organization filtering can return **zero** results even when the
  underlying text/metro/category/price query matched tools, if none of those tools' owners
  share an org with the caller — this is silent (no distinct "no visible results" vs. "no
  matching tools" signal in the response).
- `GetDownloadUrl`'s "tool is `AVAILABLE` = public" rule means a tool that is currently
  `RENTED` is **not** accessible to non-owners via this RPC, even though its images might
  still be relevant to the current renter.

## Known Discrepancies *(code vs. documentation, verified against source)*

1. **CRITICAL, FIXED 2026-07-22 — `UpdateTool` and `DeleteTool` enforced no ownership check
   at any layer.** `grpc_api_business_logic.md` documents step 1 of both as "Verify the
   current user is the owner of the tool." As found: `ToolHandler.UpdateTool`/`DeleteTool`
   (`internal/api/grpc/tool.go`) never extracted a caller identity at all;
   `toolService.UpdateTool`/`DeleteTool` (`internal/service/tool.go`) passed straight
   through to the repository; `toolRepository.Update`/`Delete`
   (`internal/repository/postgres/tool.go`) ran `UPDATE tools ... WHERE id = $N` with no
   `owner_id` predicate. **Net effect as found**: any authenticated user — no relationship
   to the tool required, not even org membership — could overwrite or soft-delete any tool
   by ID; tool IDs are sequential (`SERIAL PRIMARY KEY`), making them trivially guessable.

   **Fix applied same day**: `toolService.UpdateTool`/`DeleteTool` now fetch the tool first
   and reject with `unauthorized: only the tool owner may update/delete this tool` when
   `tool.OwnerID != callerID`, before any write. The `ToolService` interface and
   `ToolHandler` were updated to extract and thread the caller's ID via the existing
   `GetUserIDFromContext` (the same mechanism every other authenticated RPC in the codebase
   already uses).

   **Verification performed in this session**: `go build ./...` and `go vet ./...` pass;
   the full `tests/unit/...` suite passes, including a new
   `TestToolService_UpdateDelete_RequiresOwnership` covering both RPCs rejecting a
   non-owner and accepting the actual owner; the two pre-existing e2e tests for these RPCs
   (`tests/e2e/tool_test.go`) already used the tool's real owner as caller, so they are
   expected to pass unchanged. **Not verified in this session** (no live Postgres reachable
   from this sandbox): run `make test-e2e` (or `make test-integration` +`make test-e2e`)
   against your Podman DB to confirm.
2. **`GetToolImages` performs no access check, contrary to documentation.**
   `grpc_api_business_logic.md`'s "Get Tool Images" step 2 says: "Verify the tool exists
   and user has access (same logic as Get Download URL)." **As-built**,
   `imageStorageService.GetToolImages` (`internal/service/image_storage.go`) is a direct
   pass-through — `return s.toolRepo.GetImages(ctx, toolID)` — with no `userID` parameter
   at all, so it cannot apply the "owner or `AVAILABLE`" rule `GetDownloadUrl` correctly
   implements. Lower severity than Known Discrepancy 1: this is read-only metadata (image
   URLs are not directly returned, only records used to request download URLs), and its
   effective exposure is similar to treating every tool's image list as public — but it is
   still a documented check that is not implemented.
3. **A second disabled test file exists for this domain, mirroring the pattern found in
   Authentication.** `tests/e2e/search_tools_shared_org_test.go_` (trailing underscore —
   excluded from the Go build) contains `TestSearchTools_SharedOrgFiltering`. Unlike the
   Authentication domain's disabled JWT test (which has no other coverage), this specific
   behavior **is** covered at the unit-test tier by `TestToolService_SearchTools`'s
   `SharedOrgFiltering_*` subtests — so the shared-org filtering logic itself (User Story 2
   Scenario 3) is verified, just not at the e2e/full-stack level.
4. **`ListToolCategories` returns a hardcoded list, not a database query.**
   `grpc_api_business_logic.md` documents: "Return `DISTINCT` categories from the `tools`
   table." **As-built**, `toolService.ListCategories` returns a fixed 8-item Go slice
   (`"Hand Tools"`, `"Power Tools"`, etc.) regardless of what categories actually exist on
   any `tools` row. A category used by a real tool but absent from this hardcoded list
   would never appear as a filter option, and the list never reflects newly-introduced
   categories.
5. **Debug logging via `fmt.Printf` directly to stdout, inconsistent with the rest of the
   codebase.** `toolService.SearchTools` and parts of `imageStorageService.DeleteImage`
   use `fmt.Printf("DEBUG ...")` / `fmt.Printf("Warning: ...")` rather than the structured
   `internal/logger` package used consistently everywhere else (including this same
   file's sibling methods). Not a functional bug, but these lines bypass whatever log
   level/format/destination configuration the rest of the service respects.

## Current Test Coverage Baseline *(informational — grounds the next /speckit-tasks pass, not a requirement)*

Verified by inspection of `tests/unit/tool_service_test.go`, `tests/e2e/tool_test.go`,
`tests/e2e/image_storage_test.go`, `tests/integration/tool_test.go`, and the disabled
`tests/e2e/search_tools_shared_org_test.go_`:

**Covered**: `AddTool` (unit); `SearchTools`'s metro resolution and shared-org filtering,
including both the positive and filtered-out cases (unit); `ListMyTools`, the full
`UpdateTool`/`DeleteTool` happy path (e2e); the full image upload → confirm → thumbnail
pipeline and download URL access rules (e2e, `TestImageStorageService_E2E`); tool
repository CRUD (integration). **As of 2026-07-22**: `UpdateTool`/`DeleteTool` ownership
rejection and acceptance, both RPCs, via `TestToolService_UpdateDelete_RequiresOwnership`
(unit) — closes Known Discrepancy 1 at the unit level.

**Not covered anywhere**:

- **An e2e-level (real gRPC handler + live DB) non-owner-caller rejection test** for
  `UpdateTool`/`DeleteTool` — same caveat as the Organizations & Administration fix: the
  unit-level fix is verified, but nothing yet proves it through the full request stack.
- `GetToolImages` called by a caller with no relationship to the tool (Known Discrepancy
  2) — no test asserts what is or isn't visible.
- `ListToolCategories` never being cross-checked against actual `tools.categories` data
  (Known Discrepancy 4).
- `AddTool`'s direct image-URL ingestion path (Edge Cases) — only the presigned-upload
  pipeline is e2e-tested.
- `SetPrimaryImage`'s validation rejection paths (image belongs to a different tool; image
  not yet `CONFIRMED`) — its ownership rejection is covered (see below), these two are not.

**Covered as of 2026-07-22 (SBR remediation)**: `ConfirmImageUpload`/`DeleteImage`/
`SetPrimaryImage`'s ownership-rejection clauses (FR-004) and `GetDownloadUrl`'s owner-or-
`AVAILABLE` disjunctive boundary (FR-005), all previously zero-coverage — see
`TestImageStorageService_OwnershipChecks` and `TestImageStorageService_GetDownloadUrl` in
`tests/unit/image_storage_service_test.go`, and `sbr/rtm/006-tools-image-storage.rtm.md`.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: `UpdateTool` and `DeleteTool` MUST require the caller to be the tool's
  `owner_id`, verified by a fresh database read before any write. **Met as of the
  2026-07-22 fix** (Known Discrepancy 1).
- **FR-002**: `AddTool` MUST set `owner_id` from the caller's JWT, never from the request
  body.
- **FR-003**: `SearchTools` MUST resolve the search metro from the given organization when
  `organization_id` is provided, MUST require an explicit `metro` otherwise, MUST require a
  non-empty `query`, and MUST filter out any result whose owner shares no organization with
  the caller.
- **FR-004**: `GetUploadUrl`, `ConfirmImageUpload`, `DeleteImage`, and `SetPrimaryImage`
  MUST require the caller to own the target tool (or, for `ConfirmImageUpload`, to own the
  pending image record itself).
- **FR-005**: `GetDownloadUrl` MUST grant access to the tool's owner or, for any other
  caller, only when the tool's status is `AVAILABLE`.
- **FR-006**: Thumbnail generation MUST run asynchronously after `ConfirmImageUpload`
  returns, and MUST NOT block or fail the RPC response if generation fails.
- **FR-007**: `GetToolImages` MUST NOT be assumed to enforce the same access rule as
  `GetDownloadUrl` (Known Discrepancy 2) — this is the target correctness bar for a
  follow-up task, not current behavior.

### Key Entities

- **Tool**: `tools` table — `owner_id`, pricing fields (day/week/month + replacement
  cost), `duration_unit`, `condition`, `metro`, `status` (`AVAILABLE`/`RENTED`), soft-delete
  via `deleted_on`.
- **ToolImage**: `tool_images` table — `tool_id`, `user_id` (uploader), `file_path`,
  `thumbnail_path`, `status` (`PENDING`/`CONFIRMED`/`DELETED`), `is_primary` (uniquely
  enforced per confirmed tool via a partial unique index), `expires_at` (for abandoned
  pending uploads).

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: `GetToolImages` either gains the documented access check or the spec/doc is
  updated to state plainly that tool images are effectively public — Known Discrepancy 2
  does not remain a silent, undocumented gap between the two structurally similar RPCs
  (`GetDownloadUrl` and `GetToolImages`).
- **SC-002**: `ListToolCategories` either queries `DISTINCT categories` from `tools` as
  documented, or the documentation is corrected to describe the static list — Known
  Discrepancy 4 does not remain silently contradictory.
- **SC-003 — MET 2026-07-23**: An e2e-level test confirms a non-owner caller is rejected by
  `UpdateTool` and `DeleteTool` through the real gRPC handler and interceptor stack, not just
  at the service layer (`TestToolService_E2E > "UpdateTool and DeleteTool reject a non-owner
  caller"`).
- **SC-004**: A developer reading only this spec can correctly state which of the eleven
  RPCs across both services require tool ownership, which allow any authenticated caller,
  and which are effectively public.

## Assumptions

- General Rentals-domain concerns (how a rental's price snapshot is taken from a tool's
  current prices) are specified in `specs/006-rentals/spec.md`, not here.
- `docs/design/image-storage/` design artifacts were consulted for cross-reference but not
  exhaustively diffed against every implementation detail (e.g. exact presigned-URL
  parameter names) given this session's time budget; a deeper pass is reasonable future
  follow-up if this domain is revisited.
- Whether `GetToolImages` should require the `GetDownloadUrl`-style access check (Known
  Discrepancy 2) is a product decision this spec documents but does not prescribe.
