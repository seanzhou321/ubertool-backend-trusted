# Organizations + Admin Contracts

**Source of truth**: `api/proto/ubertool_trusted_backend/v1/organization_service.proto` + `admin_service.proto`

Per Constitution Principle VI (Proto-First), these .proto files define the external API surface. Generated code in `api/gen/v1/` is NEVER hand-edited.

## OrganizationService

| RPC | Request | Response | Auth | Notes |
|-----|---------|----------|------|-------|
| `CreateOrganization` | `{}` | `{ Organization }` | Any authenticated | Caller becomes SUPER_ADMIN |
| `GetOrganization` | `{ string organization_id }` | `{ Organization, member_count, my_role? }` | Member | `my_role` only if caller is member |
| `UpdateOrganization` | `{ organization_id, name?, price_threshold_cents? }` | `{ Organization }` | ADMIN/SUPER_ADMIN in org | |
| `ListMyOrganizations` | `{}` | `{ repeated OrgMembership }` | Authenticated | **FR-009**: includes `balance_cents`, `member_count` |
| `JoinOrganizationWithInvite` | `{ string token }` | `{ OrgMembership }` | Authenticated | Validates token, adds as MEMBER |
| `SetCurrentOrganization` | `{ string organization_id }` | `{ bool success }` | Authenticated | **FR-010 NEW**: validates membership, stores in Redis |
| `GetCurrentOrganization` | `{}` | `{ organization_id, organization_name }` | Authenticated | **FR-010 NEW**: reads from Redis |

### OrgMembership (extended for FR-009)
```protobuf
message OrgMembership {
    string organization_id = 1;
    string organization_name = 2;
    string metro = 3;
    int32 member_count = 4;
    string my_role = 5;              // MEMBER, ADMIN, SUPER_ADMIN
    int64 my_balance_cents = 6;      // FR-009: per-membership balance
}
```

## AdminService

| RPC | Request | Response | Auth | Notes |
|-----|---------|----------|------|-------|
| `ApproveRequestToJoin` | `{ string request_id }` | `{ OrgMembership }` | ADMIN/SUPER_ADMIN | |
| `RejectRequestToJoin` | `{ string request_id }` | `{ OrgMembership }` | ADMIN/SUPER_ADMIN | Also expires linked invitation |
| `SendInvitation` | `{ string organization_id, string email, string role }` | `{ Invitation }` | ADMIN/SUPER_ADMIN | Role: MEMBER/ADMIN |
| `AdminBlockUserAccount` | `{ string organization_id, string user_id, BlockAction action }` | `{}` | SUPER_ADMIN | BLOCK/UNBLOCK |
| `ListJoinRequests` | `{ string organization_id }` | `{ repeated JoinRequest }` | ADMIN/SUPER_ADMIN | **FR-008**: returns all requests |

## Cross-Domain Contracts (Multi-Org)

### ToolService.SearchTools (FR-011)
```protobuf
// EXTENSION to SearchToolsRequest
bool include_all_my_orgs = 10;  // When true, search all user's active orgs in same metro
```

### RentalService.CreateRentalRequest (FR-012)
```protobuf
// EXTENSION to CreateRentalRequest
string current_organization_id = 20;  // Client sends for validation

// EXTENSION to CreateRentalResponse
bool context_switch_required = 10;
string target_organization_id = 11;
string target_organization_name = 12;
```

## Redis Contract (FR-010 — Internal)
```
Key:    user:{user_id}:current_org
Value:  {organization_id}
TTL:    86400 (24h) — refreshed on activity
Ops:    SET (SetCurrentOrg), GET (GetCurrentOrg), DEL (Logout)
```

## Change Management
1. Edit `.proto` files
2. Run `make proto` (regenerates `api/gen/v1/*.pb.go`)
3. Implement handler/service changes
4. Update spec.md if behavior changes