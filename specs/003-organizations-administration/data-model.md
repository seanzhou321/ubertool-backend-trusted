# Data Model: Organizations + Admin

## Core Entities

### orgs (existing)
```sql
CREATE TABLE orgs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    metro TEXT NOT NULL,                    -- e.g., 'NYC', 'LA', 'CHI'
    price_threshold_cents BIGINT DEFAULT 0, -- Auto-notification threshold
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_orgs_metro ON orgs(metro);
CREATE INDEX idx_orgs_created_at ON orgs(created_at);
```

### users_orgs (existing — FR-009 enhanced with balance_cents)
```sql
CREATE TABLE users_orgs (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    org_id UUID NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    role TEXT NOT NULL DEFAULT 'MEMBER' CHECK (role IN ('MEMBER','ADMIN','SUPER_ADMIN')),
    balance_cents BIGINT NOT NULL DEFAULT 0,  -- FR-009: per-membership balance
    status TEXT NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE','BLOCKED','PENDING')),
    renting_blocked BOOLEAN DEFAULT FALSE,
    lending_blocked BOOLEAN DEFAULT FALSE,
    joined_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, org_id)
);

CREATE INDEX idx_users_orgs_user ON users_orgs(user_id);
CREATE INDEX idx_users_orgs_org_status ON users_orgs(org_id, status);
CREATE INDEX idx_users_orgs_org_role ON users_orgs(org_id, role);
```

### join_requests (existing)
```sql
CREATE TABLE join_requests (
    id BIGSERIAL PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    org_id UUID NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    status TEXT NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING','APPROVED','REJECTED','EXPIRED')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    resolved_at TIMESTAMPTZ
);

CREATE INDEX idx_join_requests_user ON join_requests(user_id);
CREATE INDEX idx_join_requests_org_status ON join_requests(org_id, status);
```

### invitations (existing)
```sql
CREATE TABLE invitations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    email TEXT NOT NULL,
    role TEXT NOT NULL DEFAULT 'MEMBER' CHECK (role IN ('MEMBER','ADMIN')),
    token TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_invitations_token ON invitations(token);
CREATE INDEX idx_invitations_email_org ON invitations(email, org_id);
```

## Domain Types (internal/domain/org.go)

```go
package domain

type OrgRole string

const (
    OrgRoleMember      OrgRole = "MEMBER"
    OrgRoleAdmin       OrgRole = "ADMIN"
    OrgRoleSuperAdmin  OrgRole = "SUPER_ADMIN"
)

type OrgStatus string

const (
    OrgStatusActive  OrgStatus = "ACTIVE"
    OrgStatusBlocked OrgStatus = "BLOCKED"
    OrgStatusPending OrgStatus = "PENDING"
)

type Organization struct {
    ID                   string
    Name                 string
    Metro                string
    PriceThresholdCents  int64
    CreatedAt            time.Time
    UpdatedAt            time.Time
}

type Membership struct {
    UserID         string
    OrgID          string
    Role           OrgRole
    BalanceCents   int64      // FR-009
    Status         OrgStatus
    RentingBlocked bool
    LendingBlocked bool
    JoinedAt       time.Time
}

type JoinRequest struct {
    ID        int64
    UserID    string
    OrgID     string
    Status    string    // PENDING/APPROVED/REJECTED/EXPIRED
    CreatedAt time.Time
    ResolvedAt *time.Time
}

type Invitation struct {
    ID        string
    OrgID     string
    Email     string
    Role      OrgRole
    Token     string
    ExpiresAt time.Time
    UsedAt    *time.Time
}
```

## Admin Types (internal/domain/admin.go)

```go
package domain

type BlockUserAction string

const (
    BlockActionBlock   BlockUserAction = "BLOCK"
    BlockActionUnblock BlockUserAction = "UNBLOCK"
)

type AdminBlockUserRequest struct {
    OrganizationID string
    UserID         string
    Action         BlockUserAction
}
```

## Multi-Org Query Patterns

### FR-009: Per-Membership Balance (already in users_orgs)
```sql
-- Get user's balance in specific org
SELECT balance_cents FROM users_orgs WHERE user_id = $1 AND org_id = $2 AND status = 'ACTIVE';
```

### FR-010: Current Org Context (Redis)
```
Key: user:{user_id}:current_org
Value: {org_id}
TTL: 24h (refresh on activity)
```

### FR-011: Cross-Org Search (Tools)
```sql
-- Tools from ALL user's active orgs in SAME metro as current org
SELECT t.* 
FROM tools t
JOIN orgs o ON t.org_id = o.id
JOIN users_orgs uo ON uo.org_id = o.id
WHERE uo.user_id = $1 
  AND uo.status = 'ACTIVE'
  AND o.metro = (SELECT metro FROM orgs WHERE id = $2)  -- current org's metro
  AND t.status = 'AVAILABLE';
```

### FR-012: Rental Context Switch Detection
```sql
-- Check if tool owner's org is different from current org AND user is member
SELECT t.org_id AS owner_org_id, o.name AS owner_org_name
FROM tools t
JOIN orgs o ON t.org_id = o.id
WHERE t.id = $1
  AND t.org_id != $2  -- current org
  AND EXISTS (
    SELECT 1 FROM users_orgs 
    WHERE user_id = $3 AND org_id = t.org_id AND status = 'ACTIVE'
  );
```

## gRPC Contracts

### organization_service.proto
```protobuf
service OrganizationService {
    rpc CreateOrganization(CreateOrganizationRequest) returns (CreateOrganizationResponse);
    rpc UpdateOrganization(UpdateOrganizationRequest) returns (UpdateOrganizationResponse);
    rpc GetOrganization(GetOrganizationRequest) returns (GetOrganizationResponse);
    rpc ListMyOrganizations(ListMyOrganizationsRequest) returns (ListMyOrganizationsResponse);
    rpc JoinOrganizationWithInvite(JoinOrganizationWithInviteRequest) returns (JoinOrganizationWithInviteResponse);
    rpc SetCurrentOrganization(SetCurrentOrganizationRequest) returns (SetCurrentOrganizationResponse); // FR-010
    rpc GetCurrentOrganization(GetCurrentOrganizationRequest) returns (GetCurrentOrganizationResponse); // FR-010
}

// ADD to ListMyOrganizationsResponse.Organization:
message Organization {
    string id = 1;
    string name = 2;
    string metro = 3;
    int32 member_count = 4;
    string my_role = 5;
    int64 my_balance_cents = 6;  // FR-009
}
```

### admin_service.proto
```protobuf
service AdminService {
    rpc ApproveRequestToJoin(ApproveRequestToJoinRequest) returns (ApproveRequestToJoinResponse);
    rpc RejectRequestToJoin(RejectRequestToJoinRequest) returns (RejectRequestToJoinResponse);
    rpc SendInvitation(SendInvitationRequest) returns (SendInvitationResponse);
    rpc AdminBlockUserAccount(AdminBlockUserAccountRequest) returns (AdminBlockUserAccountResponse);
    rpc ListJoinRequests(ListJoinRequestsRequest) returns (ListJoinRequestsResponse); // FR-008
}
```

### tool_service.proto (FR-011 extension)
```protobuf
// ADD to SearchToolsRequest:
bool include_all_my_orgs = 10;
```

### rental_service.proto (FR-012 extension)
```protobuf
// ADD to CreateRentalRequest:
string current_organization_id = 20;

// ADD to CreateRentalResponse:
bool context_switch_required = 10;
string target_organization_id = 11;
string target_organization_name = 12;
```

## Proto Regeneration
```bash
make proto
# Generates: api/gen/v1/organization_service.pb.go, admin_service.pb.go
# NEVER hand-edit generated files
```