# Data Model: Tools + Image Storage

> **Correction 2026-07-28**: This document's `tools` table and `Tool` struct below do not match
> the actual schema (`podman/trusted-group/postgres/ubertool_schema_trusted.sql`) — notably the ID
> types (real schema uses `SERIAL`/`INTEGER`, not `UUID`), the `condition`/`status` enum values, and
> most importantly **`owner_org_id` does not exist and must not be added**. A tool is owned by a
> user (`tools.owner_id`) and scoped to a metro (`tools.metro`), never bound to a single
> organization — see `docs/design/multi-org.md` for why. Treat the real schema file and
> `internal/domain/tool.go` as ground truth over this table; the shape below is retained only for
> the non-org-related design discussion elsewhere in this doc.

## Entities

### tools (existing)
| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | UUID | PK | Tool ID |
| owner_id | UUID | NOT NULL, FK → users.id | Owner user |
| name | TEXT | NOT NULL | Tool name |
| description | TEXT | | |
| categories | TEXT[] | | Category tags |
| daily_price_cents | BIGINT | NOT NULL | Price per day |
| weekly_price_cents | BIGINT | | Price per week |
| monthly_price_cents | BIGINT | | Price per month |
| replacement_cost_cents | BIGINT | NOT NULL | For damage calculation |
| condition | TEXT | NOT NULL, CHECK IN ('NEW','LIKE_NEW','GOOD','FAIR') | Condition |
| metro | TEXT | NOT NULL | Metro area (e.g., "NYC") |
| duration_unit | TEXT | NOT NULL, CHECK IN ('DAY','WEEK','MONTH') | Rental duration unit |
| status | TEXT | NOT NULL, CHECK IN ('AVAILABLE','RENTED','MAINTENANCE','RETIRED') | Availability |
| created_at | TIMESTAMPTZ | NOT NULL DEFAULT now() | |
| updated_at | TIMESTAMPTZ | NOT NULL DEFAULT now() | |

**Indexes**: `(owner_id)`, `(metro, status)`, `(status, created_at)`

### tool_images (existing)
| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | UUID | PK | Image ID |
| tool_id | UUID | NOT NULL, FK → tools.id | Parent tool |
| file_name | TEXT | NOT NULL | Original filename |
| file_path | TEXT | NOT NULL | S3/GCS path |
| thumbnail_path | TEXT | | Thumbnail path |
| is_primary | BOOLEAN | DEFAULT FALSE | Primary image flag |
| created_at | TIMESTAMPTZ | NOT NULL DEFAULT now() | |

**Indexes**: `(tool_id)`, `(tool_id, is_primary)`

## Domain Types (internal/domain/tool.go)

```go
package domain

type ToolStatus string

const (
    ToolStatusAvailable  ToolStatus = "AVAILABLE"
    ToolStatusRented     ToolStatus = "RENTED"
    ToolStatusMaintenance ToolStatus = "MAINTENANCE"
    ToolStatusRetired    ToolStatus = "RETIRED"
)

type Tool struct {
    ID                     string
    OwnerID                string
    Name                   string
    Description            string
    Categories             []string
    DailyPriceCents        int64
    WeeklyPriceCents       int64
    MonthlyPriceCents      int64
    ReplacementCostCents   int64
    Condition              string
    Metro                  string
    DurationUnit           string
    Status                 ToolStatus
    CreatedAt              time.Time
    UpdatedAt              time.Time
}

type ToolImage struct {
    ID             string
    ToolID         string
    FileName       string
    FilePath       string
    ThumbnailPath  string
    IsPrimary      bool
    CreatedAt      time.Time
}
```

## gRPC Contracts

### tool_service.proto (existing + FR-008, FR-009)

```protobuf
message SearchToolsRequest {
  // ... existing fields ...
  string query = 1;
  string metro = 2;
  string category = 3;
  int32 max_daily_price_cents = 4;
  int32 min_daily_price_cents = 5;
  bool available_only = 6;
  int32 page_size = 7;
  string page_token = 8;
  
  // NEW FR-008
  bool include_all_my_orgs = 10;  // When true: search across ALL user's active orgs in same metro
}

message SearchToolsResponse {
  repeated Tool tools = 1;
  string next_page_token = 2;
}

message Tool {
  string id = 1;
  string owner_id = 2;
  string name = 3;
  // ... existing fields ...
  string metro = 10;
  string status = 11;
  User owner = 12;  // FR-009: owner.orgs (pre-existing User.orgs field) = shared orgs with requester — no new field added
}
```
No `owner_organizations`/`OrganizationSummary` field was added — see the correction note at the
top of `contracts/README.md`.

### image_storage_service.proto (unchanged)

```protobuf
service ImageStorageService {
  rpc GetUploadUrl(GetUploadUrlRequest) returns (GetUploadUrlResponse);
  rpc ConfirmImageUpload(ConfirmImageUploadRequest) returns (ConfirmImageUploadResponse);
  rpc DeleteImage(DeleteImageRequest) returns (DeleteImageResponse);
  rpc SetPrimaryImage(SetPrimaryImageRequest) returns (SetPrimaryImageResponse);
  rpc GetDownloadUrl(GetDownloadUrlRequest) returns (GetDownloadUrlResponse);
  rpc GetToolImages(GetToolImagesRequest) returns (GetToolImagesResponse);
}
```

## Multi-Org Queries

> Corrected 2026-07-28: tools have no `owner_org_id`/org column and there is no server-side
> "current org" to resolve a metro from (see `docs/design/multi-org.md`). Cross-org visibility is
> metro-based plus a per-tool shared-org post-filter, implemented in Go
> (`toolService.SearchTools`, `internal/service/tool.go`), not a single SQL join.

### FR-008: Cross-Org Search (metro-scoped, not org-scoped)
```sql
-- Base filter: tools in the given metro, excluding the requester's own tools
SELECT t.*
FROM tools t
WHERE t.metro = $1  -- explicit metro param, or the metro of an org the caller names
  AND t.deleted_on IS NULL
  AND t.owner_id != $2
  AND t.status != 'UNAVAILABLE'
  AND ($3 = '' OR t.name ILIKE '%' || $3 || '%')
ORDER BY t.price_per_day_cents ASC
LIMIT $4 OFFSET $5;
```

### FR-009: Shared-Org Owner Filtering (per tool, in Go)
```sql
-- For each candidate tool, exclude it unless the owner and requester share at least one active org
SELECT DISTINCT o.id, o.name, o.metro
FROM orgs o
JOIN users_orgs uo_requester ON uo_requester.org_id = o.id 
    AND uo_requester.user_id = $requester_id AND uo_requester.status = 'ACTIVE'
JOIN users_orgs uo_owner ON uo_owner.org_id = o.id 
    AND uo_owner.user_id = $owner_id AND uo_owner.status = 'ACTIVE';
-- Empty result set => exclude the tool from search results entirely.
```

## Proto Regeneration
```bash
make proto
# api/gen/v1/tool_service.pb.go, image_storage_service.pb.go
```