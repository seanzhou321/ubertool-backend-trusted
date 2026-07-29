# Tools + Image Storage Contracts

Source: `api/proto/ubertool_trusted_backend/v1/tool_service.proto` + `image_storage_service.proto`

> **Correction 2026-07-28**: `owner_org_id` and any server-side "current org" (Redis) were both
> reverted — see `docs/design/multi-org.md`. `AddTool` sets `owner_id` from the JWT only; tools
> are never bound to an org. `SearchTools`/`ListMyTools`/`GetTool` cross-org behavior is driven by
> the caller's explicit `metro`/`organization_id` request parameters and a per-tool shared-org
> post-filter (`getSharedOrganizations`), not an implicit cached "current org."
>
> **Correction 2026-07-29**: There is no `owner_organizations`/`OrganizationSummary` field on
> `Tool`, and none was ever needed. The real `Tool` message (`api/proto/.../tool_service.proto`)
> already has `User owner = 11`, and `User` (`ubertool_schema.proto`) already has
> `repeated Organization orgs = 6`. FR-009 is implemented by populating that pre-existing
> `owner.orgs` field with the shared orgs between the tool's owner and the requester
> (`toolService.populateToolOwner`/`getSharedOrganizations`, `internal/service/tool.go`) — no
> proto change was needed at all. `CreateRentalRequestResponse.shared_organization_ids`/
> `shared_organization_names` (005-rentals) were removed for the same reason: this is where org
> discovery belongs, not the rental-creation response.

## ToolService

| RPC | Request | Response | Notes |
|-----|---------|----------|-------|
| `AddTool` | `AddToolRequest { Tool }` | `AddToolResponse { Tool }` | `owner_id` from JWT only — no org association at all |
| `GetTool` | `GetToolRequest { string id }` | `GetToolResponse { Tool }` | **FR-009**: `tool.owner.orgs` filtered to shared orgs (existing `User.orgs` field, not a new one) |
| `UpdateTool` | `UpdateToolRequest { string id, Tool }` | `UpdateToolResponse { Tool }` | Requires ownership (FR-001) |
| `DeleteTool` | `DeleteToolRequest { string id }` | `DeleteToolResponse {}` | Requires ownership |
| `SearchTools` | `SearchToolsRequest` | `SearchToolsResponse { repeated Tool, next_page_token }` | **FR-008**: metro-scoped + per-tool shared-org filter (no `include_all_my_orgs` flag needed — there's no single-org scope to expand from) |
| `ListMyTools` | `ListMyToolsRequest {}` | `ListMyToolsResponse { repeated Tool }` | Caller's own tools — org-independent, since tools aren't org-scoped |

### Tool Message (FR-009 — no new fields, reuses existing `User.orgs`)

The real `Tool` message already has an `owner` field typed as the shared `User` message
(`api/proto/.../ubertool_schema.proto`), which already has `repeated Organization orgs = 6`.
FR-009 populates that field with the shared orgs rather than adding a parallel
`owner_organizations`/`OrganizationSummary` field:

```protobuf
// ubertool_schema.proto (existing, unchanged)
message User {
  int32 id = 1;
  string name = 2;
  string email = 3;
  string phone = 4;
  string avatar_url = 5;
  repeated Organization orgs = 6;  // FR-009: populated with shared orgs when owner is a Tool.owner
  string created_on = 7;
}

// tool_service.proto (existing, unchanged)
message Tool {
  int32 id = 1;
  // ...
  User owner = 11;  // owner.orgs = shared orgs between owner and requester (FR-009)
  // ...
}
```

### SearchToolsRequest (FR-008)

```protobuf
message SearchToolsRequest {
  string query = 1;
  string metro = 2;
  string category = 3;
  int64 max_daily_price_cents = 4;
  int64 min_daily_price_cents = 5;
  bool available_only = 6;
  int32 page_size = 7;
  string page_token = 8;
  // No include_all_my_orgs flag: search is always metro-scoped (explicit `metro`, or the metro
  // of an `organization_id` the caller names) plus the per-tool shared-org post-filter — there is
  // no single-org "default scope" to opt out of expanding.
}
```

## ImageStorageService (unchanged)

| RPC | Request | Response | Notes |
|-----|---------|----------|-------|
| `GetUploadUrl` | `GetUploadUrlRequest { string tool_id, string file_name, string content_type }` | `GetUploadUrlResponse { string upload_url, string image_id }` | Presigned S3 URL |
| `ConfirmImageUpload` | `ConfirmImageUploadRequest { string image_id, string tool_id }` | `ConfirmImageUploadResponse { ToolImage }` | Triggers thumbnail generation |
| `DeleteImage` | `DeleteImageRequest { string image_id }` | `DeleteImageResponse {}` | Owner only |
| `SetPrimaryImage` | `SetPrimaryImageRequest { string tool_id, string image_id }` | `SetPrimaryImageResponse { ToolImage }` | Owner only |
| `GetDownloadUrl` | `GetDownloadUrlRequest { string image_id }` | `GetDownloadUrlResponse { string download_url }` | Owner or renter (FR-005) |
| `GetToolImages` | `GetToolImagesRequest { string tool_id }` | `GetToolImagesResponse { repeated ToolImage }` | No access control (FR-007) |

## Multi-Org Semantics

| Feature | Parameter | Behavior |
|---------|-----------|----------|
| `AddTool` | (implicit) | `owner_id` = JWT caller only; no org is recorded at all |
| `SearchTools` | `metro` / `organization_id` (explicit) | Metro-scoped base filter, then excludes tools whose owner shares zero active orgs with the requester; each result's `owner.orgs` = that shared-org set |
| `GetTool` | (implicit) | `owner.orgs` = intersection of requester's orgs ∩ owner's orgs (reuses `User.orgs`, not a new field) |
| `ListMyTools` | (implicit) | Caller's own tools by `owner_id`, independent of any org |

## Proto Changes Required

None. FR-008/FR-009 reuse existing fields (`metro`, `organization_id`, `User.orgs`).

## No Breaking Changes

No proto changes were made for FR-008/FR-009.