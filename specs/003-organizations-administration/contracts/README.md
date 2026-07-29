# Organizations + Admin Contracts

**Source of truth**: `api/proto/ubertool_trusted_backend/v1/organization_service.proto` + `admin_service.proto`

Per Constitution Principle VI (Proto-First), these .proto files define the external API surface. Generated code in `api/gen/v1/` is NEVER hand-edited.

> **Correction 2026-07-28**: `SetCurrentOrganization`/`GetCurrentOrganization` (FR-010) and the
> Redis-backed "current org" they depended on were both removed from the actual proto and
> service. A user's "current organization" is a client/device-local UI preference — a user may
> have several devices, each focused on a different org — so the server has nothing to cache per
> user and must not persist one. `include_all_my_orgs` (FR-011) and `current_organization_id`/
> `context_switch_required` (FR-012) were never added either; see `docs/design/multi-org.md` for
> the corrected design that was actually implemented. The sections below documenting those are
> retained struck through for history — do not reintroduce them.
>
> **Correction 2026-07-29**: `CreateOrganization`'s "Any authenticated" auth column below
> describes the token requirement only. Reachability is additionally gated by the
> `features.allow_api_organization_creation` config flag (`internal/config/config.go`,
> enforced in `internal/api/grpc/org.go`) — `false` in production
> (`config/config.ec2.prod.yaml`), where new orgs are provisioned by the backend team
> directly rather than self-service via the API. The flag is `true` in every
> non-production config.

## OrganizationService

| RPC | Request | Response | Auth | Notes |
|-----|---------|----------|------|-------|
| `CreateOrganization` | `{}` | `{ Organization }` | Any authenticated, **and** `features.allow_api_organization_creation=true` | Caller becomes SUPER_ADMIN; disabled in production |
| `GetOrganization` | `{ string organization_id }` | `{ Organization, member_count, my_role? }` | Member | `my_role` only if caller is member |
| `UpdateOrganization` | `{ organization_id, name?, price_threshold_cents? }` | `{ Organization }` | ADMIN/SUPER_ADMIN in org | |
| `ListMyOrganizations` | `{}` | `{ repeated OrgMembership }` | Authenticated | **FR-009**: includes `balance_cents`, `member_count` |
| `JoinOrganizationWithInvite` | `{ string token }` | `{ OrgMembership }` | Authenticated | Validates token, adds as MEMBER |
| ~~`SetCurrentOrganization`~~ | — | — | — | REMOVED (FR-010) — no server-side "current org"; see correction note above |
| ~~`GetCurrentOrganization`~~ | — | — | — | REMOVED (FR-010) — no server-side "current org"; see correction note above |

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

## Cross-Domain Contracts (Multi-Org) — As Actually Built

### ToolService.SearchTools / GetTool (FR-011, FR-009) — where org discovery actually lives
No `include_all_my_orgs` field exists or was needed. `SearchToolsRequest` already carries `metro`
(and `organization_id` to resolve one) as explicit fields; `toolService.SearchTools` filters by
`tools.metro` (tools are never org-scoped) then post-filters per-tool by shared active orgs
between the owner and requester (`getSharedOrganizations`, `internal/service/tool.go`). Each
returned `Tool.owner` (a `User` message) has its pre-existing `orgs` field (`User.orgs`,
`ubertool_schema.proto`) populated with exactly those shared orgs — this is how the client learns
*which* `organization_id` values are valid for renting a given tool, before ever calling
`CreateRentalRequest`. No new proto field was needed; `GetTool` does the same via `populateToolOwner`.

### RentalService.CreateRentalRequest (FR-012 / Rentals FR-008) — validation only, not discovery
No `current_organization_id`/`context_switch_required`/`target_organization_id` fields exist or
were needed. `CreateRentalRequestRequest.organization_id` (already existing) is the caller's
explicit choice — the renter already knows a valid value from `Tool.owner.orgs` above.
`rentalService.CreateRentalRequest` (`internal/service/rental.go`) validates it via
`isSharedOrganization` and, on mismatch, returns a plain `FAILED_PRECONDITION` (human-readable
message only). `CreateRentalRequestResponse.shared_organization_ids`/`shared_organization_names`
were added, found to duplicate `Tool.owner.orgs` in the wrong layer (a write-result response, not
a discovery surface), and removed 2026-07-29.

## No Redis Contract

There is no Redis dependency anywhere in this service. "Current organization" is a client/device-
local UI preference, never cached or persisted server-side — see `docs/design/multi-org.md`.

## Change Management
1. Edit `.proto` files
2. Run `make proto` (regenerates `api/gen/v1/*.pb.go`)
3. Implement handler/service changes
4. Update spec.md if behavior changes