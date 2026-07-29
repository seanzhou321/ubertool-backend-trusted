# Multi-Organization Design (Ground Truth)

> **Status**: This is the authoritative, implemented design. A prior attempt added an `owner_org_id`
> field to `tools` plus a Redis-cached "current organization" server-side session concept
> (see `docs/design/scratchpad/owner_org_id-for-multi-org.md`); both were reverted as of
> 2026-07-28 because they broke the core guarantees below. `tools` has no `owner_org_id` column,
> and the server does not cache or persist a user's "current organization" anywhere — that concept
> is a client/device-local UI preference only (a user may have several devices, each focused on a
> different org). Every request that needs an organization context passes `organization_id`
> explicitly (e.g. `CreateRentalRequestRequest.organization_id`).

---

## Core Principles

### 1. Tools Belong to Users, Not Organizations
- A tool is owned by a **user** (`tools.owner_id → users.id`)
- A tool exists in a **metro** (`tools.metro`) for geographic discovery
- A tool is **not** intrinsically bound to any single organization
- A user may belong to multiple organizations (`users_orgs`)

### 2. Organizations Provide Context, Not Ownership
- An organization (Church/Group) defines:
  - A **metro** (for auto-filtering search)
  - A **billing/ledger context** (balances, bill-splitting)
  - A **membership roster** (who can interact within this context)
- Organizations do **not** own tools; users do

### 3. Rentals Have an Organization Context (Chosen at Request Time)
- When a renter creates a rental request, they specify `organization_id`
- This `organization_id` becomes the rental's `org_id` (FK to `orgs`)
- The rental's `org_id` determines:
  - Which ledger the transaction posts to
  - Which bill-split rules apply
  - Which admin policies govern the rental
- The both tool owner and the renter must be the members of the rental's `org_id`

---

## Search & Discovery Flow

### User Journey (from `grpc_api_business_logic.md` §3.3)

1. **User selects "Current Org"** (e.g., Church A) as their active search context — this is
   purely client/device-local UI state (see the Status note at the top of this doc); the client
   passes it as `SearchToolsRequest.organization_id`, the server never stores it
2. **Auto-Metro Filter**: Search automatically filters for tools in Church A's `metro`
3. **Cross-Org Results**: Results **also include** tools from *other* organizations the user belongs to (e.g., Church B), provided they share the same/compatible metro
4. **Results Display**: Unified list sorted by price (low → high), no map view; each tool's `owner.orgs` tells the client which orgs it could rent that specific tool under
5. **Context Switch Prompt (Soft, UI-Level — client state only)**:
   > *"The owner of this tool is in [Church B]. Switch your view to that organization to rent it?"*
   The client reads the shared org(s) straight off `owner.orgs` (no RPC call needed to discover
   this) and, if the renter confirms, simply calls `CreateRentalRequest` with
   `organization_id = "<church-b-id>"` — there is no server-side "switch" RPC to call first.

### Search Implementation (`tool.go:SearchTools`)
- Input: `userID`, optional `orgID`, `metro`, `query`, filters
- If `orgID` provided:
  - Verify user is member of that org
  - Use org's `metro` for filtering
- If `orgID` not provided:
  - Require explicit `metro` parameter
- Filter: `tools.metro = searchMetro` AND `tools.deleted_on IS NULL` AND `tools.owner_id != userID` AND `tools.status != 'UNAVAILABLE'`
- **Post-filter**: For each tool, compute shared organizations between tool owner and requesting user (`getSharedOrganizations`)
- **Exclude** tools where owner shares **zero** organizations with requester
- **Blocked-flag rule (2026-07-29)**: an org only counts as "shared" if the owner is not
  `lending_blocked` there and the requester is not `renting_blocked` there — each flag is checked
  against the role that party actually has in a prospective rental (owner lends, requester rents).
  `users_orgs.renting_blocked`/`lending_blocked` are independent per-role flags (e.g. set by
  `BillSplitService` for an unpaid bill), so a user blocked from one side of the marketplace in an
  org must still be able to use the other side there. Applied identically in
  `rentalService.isSharedOrganization`/`getSharedOrganizations` (`internal/service/rental.go`) for
  the actual rental-creation check.
- **Org discovery lives here**: each returned tool's `owner` field (`toolService.populateToolOwner`)
  has its `Orgs` populated with exactly the shared orgs — this serializes over the wire as
  `Tool.owner.orgs`, reusing the pre-existing `User.orgs` proto field (no new field was added).
  `GetTool` does the same. This is how the client learns which `organization_id` values are valid
  for a given tool *before* ever calling `CreateRentalRequest` — see "Rental Workflow" below.

---

## Rental Workflow

### Create Rental Request (`rental.go:CreateRentalRequest`)
**Input**: `tool_id`, `start_date`, `end_date`, `organization_id` (chosen by renter)

**Business Logic**:
1. Verify renter is member of `organization_id`
2. Fetch tool; verify availability
3. Calculate cost using tool's price snapshot (tiered pricing algorithm)
4. Create rental with:
   - `org_id = organization_id` (renter's chosen context)
   - `owner_id = tool.owner_id`
   - `renter_id = caller`
   - Price snapshot copied from tool
5. Notify owner (email + in-app + push)

### Example Scenario
- **User Alice** belongs to: Church A (metro: North), Church B (metro: North)
- **User Bob** belongs to: Church B (metro: North), Church C (metro: South)
- Bob lists a **Drill** (metro: North, owner: Bob)
- Alice (viewing Church A context) searches → sees Bob's Drill (shared metro: North, shared org: Church B)
- Alice clicks "Rent" → UI prompts: *"Owner is in Church B. Switch context to Church B?"*
- Alice confirms → `CreateRentalRequest(tool_id=Drill, organization_id=Church_B)`
- Rental created with `org_id = Church_B`, billed to Church B's ledger

---

## Billing & Ledger

- Each organization has its own **ledger** (`ledger_transactions.org_id`)
- User balances are **per-organization** (`users_orgs.balance_cents` scoped by `org_id`)
- Bill-splitting runs **per-organization** (`bills.org_id`)
- When a rental completes with `charge_billsplit=true`:
  - Owner credited in `rental.org_id` ledger
  - Renter debited in `rental.org_id` ledger
- If `charge_billsplit=false`: No ledger entries; parties settle directly

---

## Why This Design Works

| Requirement | How It's Satisfied |
|-------------|-------------------|
| User in multiple orgs lists tool once | Tool owned by user, not org; visible across all shared orgs |
| Cross-org rental | Renter picks rental org at request time from shared orgs |
| Metro-based discovery | Tool has `metro`; search filters by metro of active org |
| Clear billing context | Rental's `org_id` explicitly chosen → unambiguous ledger |
| Admin control per org | Admins manage members, blocks, billsplit in their org only |
| No duplicate tool listings | Single tool row serves all orgs the owner participates in |

---

## Related Artifacts

- `docs/design/PRD-Backend.md` — Product requirements (invite-only, ledger, multi-org)
- `docs/design/Architecture-Design.md` — Service breakdown, data flows
- `docs/design/grpc_api_business_logic.md` — Detailed API business logic (search, rental, billing)
- `docs/design/tool-rental-pricing-algorithm.md` — Tiered pricing used in rental creation

> **Resolved**: `rentalService.CreateRentalRequest` (`internal/service/rental.go`) validates the
> caller's requested `organization_id` by checking both the renter and the tool owner are active
> members of that org (`isSharedOrganization`/`getSharedOrganizations`), and the `rentals` table
> enforces the same invariant at the DB layer via `rentals_shared_org_check`. No tool-to-org
> binding was needed.
>
> **Resolved (2026-07-29)**: `CreateRentalRequestResponse` does not carry a shared-org list
> (`shared_organization_ids`/`shared_organization_names` were added, then removed) — a
> rental-creation response is a write-result, not a discovery surface. Org discovery lives at
> `SearchTools`/`GetTool` time instead, via the pre-existing `Tool.owner.orgs` field (see "Search
> & Discovery Flow" above); `CreateRentalRequest`'s shared-org rejection is only a defensive
> backstop for the rare case where membership changed between search and request.
