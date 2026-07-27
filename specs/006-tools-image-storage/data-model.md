# Data Model: Tools + Image Storage

## Entities

### tools (existing)
| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | UUID | PK | Tool ID |
| owner_id | UUID | NOT NULL, FK → users.id | Owner user |
| owner_org_id | UUID | NOT NULL, FK → orgs.id | **Owner's organization** |
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

**Indexes**: `(owner_id)`, `(owner_org_id)`, `(metro, status)`, `(status, created_at)`

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
    OwnerOrgID             string
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
  
  // NEW FR-009: Owner's organizations SHARED with requester
  repeated OrganizationSummary owner_organizations = 20;
}

message OrganizationSummary {
  string id = 1;
  string name = 2;
  string metro = 3;
}
```

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

### FR-008: Cross-Org Search (include_all_my_orgs=true)
```sql
SELECT t.*
FROM tools t
JOIN orgs o ON t.owner_org_id = o.id
JOIN users_orgs uo ON o.id = uo.org_id
WHERE uo.user_id = $1
  AND uo.status = 'ACTIVE'
  AND o.metro = $2  -- resolved from current org or first active org
  AND t.status = 'AVAILABLE'
  AND ($3 = '' OR t.name ILIKE '%' || $3 || '%')
ORDER BY t.created_at DESC
LIMIT $4 OFFSET $5;
```

### FR-009: Shared-Org Owner Filtering
```sql
SELECT DISTINCT o.id, o.name, o.metro
FROM orgs o
JOIN users_orgs uo_requester ON uo_requester.org_id = o.id 
    AND uo_requester.user_id = $requester_id AND uo_requester.status = 'ACTIVE'
JOIN users_orgs uo_owner ON uo_owner.org_id = o.id 
    AND uo_owner.user_id = $owner_id AND uo_owner.status = 'ACTIVE'
WHERE o.id = $tool_owner_org_id;  -- or all owner's orgs
```

## Proto Regeneration
```bash
make proto
# api/gen/v1/tool_service.pb.go, image_storage_service.pb.go
```