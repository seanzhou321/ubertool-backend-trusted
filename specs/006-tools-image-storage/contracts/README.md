# Tools + Image Storage Contracts

Source: `api/proto/ubertool_trusted_backend/v1/tool_service.proto` + `image_storage_service.proto`

## ToolService

| RPC | Request | Response | Notes |
|-----|---------|----------|-------|
| `AddTool` | `AddToolRequest { Tool }` | `AddToolResponse { Tool }` | `owner_id` from JWT; `owner_org_id` from current org (FR-010) |
| `GetTool` | `GetToolRequest { string id }` | `GetToolResponse { Tool }` | **FR-009**: `owner_organizations` filtered to shared orgs |
| `UpdateTool` | `UpdateToolRequest { string id, Tool }` | `UpdateToolResponse { Tool }` | Requires ownership (FR-001) |
| `DeleteTool` | `DeleteToolRequest { string id }` | `DeleteToolResponse {}` | Requires ownership |
| `SearchTools` | `SearchToolsRequest` | `SearchToolsResponse { repeated Tool, next_page_token }` | **FR-008**: `include_all_my_orgs` flag |
| `ListMyTools` | `ListMyToolsRequest {}` | `ListMyToolsResponse { repeated Tool }` | Owner's tools in current org |

### Tool Message (FR-008, FR-009 extensions)

```protobuf
message Tool {
  string id = 1;
  string owner_id = 2;
  string owner_org_id = 3;        // Owner's primary org for this tool
  string name = 4;
  string description = 5;
  repeated string categories = 6;
  int64 daily_price_cents = 7;
  int64 weekly_price_cents = 8;
  int64 monthly_price_cents = 9;
  int64 replacement_cost_cents = 10;
  string condition = 11;
  string metro = 12;
  string duration_unit = 13;       // DAY, WEEK, MONTH
  string status = 14;              // AVAILABLE, RENTED, MAINTENANCE, RETIRED
  repeated ToolImage images = 15;
  string created_at = 16;
  string updated_at = 17;
  
  // NEW FR-009: Owner's organizations SHARED with requester
  repeated OrganizationSummary owner_organizations = 20;
}

message OrganizationSummary {
  string id = 1;
  string name = 2;
  string metro = 3;
}
```

### SearchToolsRequest (FR-008 extension)

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
  
  // NEW FR-008
  bool include_all_my_orgs = 10;   // Default false: search current org only
                                   // True: search ALL user's active orgs in same metro
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
| `AddTool` | (implicit) | `owner_org_id` = Redis `user:{id}:current_org` |
| `SearchTools` | `include_all_my_orgs=true` | Search all ACTIVE orgs in current org's metro |
| `GetTool` | (implicit) | `owner_organizations` = intersection of requester's orgs ∩ owner's orgs |
| `ListMyTools` | (implicit) | Filters by `tools.owner_org_id = current_org` |

## Proto Changes Required

1. `tool_service.proto`: Add `include_all_my_orgs` to `SearchToolsRequest`
2. `tool_service.proto`: Add `owner_organizations` to `Tool` message
3. Regenerate: `make proto`

## No Breaking Changes

All new fields are optional with defaults (`false`, empty list). Existing clients work unchanged.