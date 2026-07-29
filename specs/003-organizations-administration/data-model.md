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

### FR-010: Current Org Context — REMOVED
A prior attempt cached the user's "current organization" server-side in Redis
(`user:{user_id}:current_org`, 24h TTL). This was reverted: which org a user is "focused on" is a
client/device-local UI preference, not server state — a user may have several devices, each
focused on a different org, so the server has no single "current org" to cache per user. Every
request that needs an organization context passes `organization_id` explicitly instead (e.g.
`CreateRentalRequestRequest.organization_id`). There is no Redis dependency in this service. See
`docs/design/multi-org.md`.

### FR-011: Cross-Org Search (Tools)
`tools` has no `org_id`/`owner_org_id` column — a tool is owned by a user (`tools.owner_id`), not
an org, and is metro-scoped (`tools.metro`), not org-scoped. `toolService.SearchTools`
(`internal/service/tool.go`) filters by `tools.metro` + status, then post-filters by computing the
shared active organizations between the tool owner and the requesting user
(`getSharedOrganizations`), excluding tools where they share zero orgs:
```sql
-- Base filter (metro-scoped, not org-scoped)
SELECT * FROM tools
WHERE metro = $1 AND deleted_on IS NULL AND owner_id != $2 AND status != 'UNAVAILABLE';
```
```sql
-- Per-tool post-filter (Go, not SQL): shared active orgs between owner and requester
SELECT o.* FROM orgs o
JOIN users_orgs uo1 ON uo1.org_id = o.id AND uo1.user_id = $owner_id AND uo1.status = 'ACTIVE'
JOIN users_orgs uo2 ON uo2.org_id = o.id AND uo2.user_id = $requester_id AND uo2.status = 'ACTIVE';
```

### FR-012: Rental Org-Context Validation
No `owner_org_id` on tools, so there is no tool-to-org binding to compare against. Instead,
`rentalService.CreateRentalRequest` (`internal/service/rental.go`) validates the caller-supplied
`organization_id` directly: both the renter and the tool owner must be active members of that org
(`isSharedOrganization`); on mismatch it returns `FAILED_PRECONDITION` with the caller's list of
orgs shared with the owner (`getSharedOrganizations`). The `rentals` table also enforces this
invariant at the DB layer via the `rentals_shared_org_check` CHECK constraint.

## gRPC Contracts

### organization_service.proto
```protobuf
service OrganizationService {
    rpc CreateOrganization(CreateOrganizationRequest) returns (CreateOrganizationResponse);
    rpc UpdateOrganization(UpdateOrganizationRequest) returns (UpdateOrganizationResponse);
    rpc GetOrganization(GetOrganizationRequest) returns (GetOrganizationResponse);
    rpc ListMyOrganizations(ListMyOrganizationsRequest) returns (ListMyOrganizationsResponse);
    rpc JoinOrganizationWithInvite(JoinOrganizationWithInviteRequest) returns (JoinOrganizationWithInviteResponse);
    // FR-010 (SetCurrentOrganization/GetCurrentOrganization) was REMOVED — no server-side
    // "current org" exists. See docs/design/multi-org.md.
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