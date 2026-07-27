# Ledger Service Contracts

This directory references the protobuf contract for the Ledger service.

## Source of Truth
**`api/proto/ubertool_trusted_backend/v1/ledger_service.proto`**

Per Constitution Principle VI (Proto-First API Contract), this is the single source of truth for the external API surface.

## Contract Summary

### Service: `LedgerService`
| RPC | Request | Response | Notes |
|-----|---------|----------|-------|
| `GetBalance` | `GetBalanceRequest { int32 organization_id }` | `GetBalanceResponse { int32 balance, string last_updated_on }` | `organization_id=0` → cross-org rollup (FR-004) |
| `GetTransactions` | `GetTransactionsRequest { int32 organization_id, int32 page, int32 page_size }` | `GetTransactionsResponse { repeated Transaction, int32 total_count }` | Paginated |
| `GetLedgerSummary` | `GetLedgerSummaryRequest { int32 organization_id, int32 number_of_months }` | `GetLedgerSummaryResponse { int32 balance, map<string,int32> status_count }` | `organization_id=0` → cross-org rollup (FR-004) |

### Messages

#### `GetBalanceRequest`
- `organization_id` (int32): Target organization. **`0` = all active orgs for user** (FR-004 sentinel)

#### `GetBalanceResponse`
- `balance` (int32): Balance in cents (positive = credit, negative = debit)
- `last_updated_on` (string): Date string `YYYY-MM-DD`

#### `GetTransactionsRequest`
- `organization_id` (int32): Target organization
- `page` (int32): 1-based page number
- `page_size` (int32): Items per page

#### `GetTransactionsResponse`
- `transactions` (repeated `Transaction`): Transaction list
- `total_count` (int32): Total matching transactions

#### `Transaction`
- `id` (int32): Transaction ID
- `user_id` (int32): User ID
- `organization_id` (int32): Organization ID
- `amount` (int32): Amount in cents (+credit, -debit)
- `type` (TransactionType): Type enum
- `related_rental` (RentalRequest): Optional linked rental
- `description` (string): Human-readable description
- `charged_on` (string): Date `YYYY-MM-DD`

#### `TransactionType` enum
| Value | Name | Description |
|-------|------|-------------|
| 0 | `TRANSACTION_TYPE_UNSPECIFIED` | Invalid |
| 1 | `TRANSACTION_TYPE_RENTAL_DEBIT` | Rental payment (outgoing) |
| 2 | `TRANSACTION_TYPE_LENDING_CREDIT` | Lending income (incoming) |
| 3 | `TRANSACTION_TYPE_LENDING_DEBIT` | Lending expense |
| 4 | `TRANSACTION_TYPE_REFUND` | Refund received |
| 5 | `TRANSACTION_TYPE_ADJUSTMENT` | Manual adjustment |

#### `GetLedgerSummaryRequest`
- `organization_id` (int32): Target organization. **`0` = all active orgs** (FR-004)
- `number_of_months` (int32): Lookback window (unused in current impl)

#### `GetLedgerSummaryResponse`
- `balance` (int32): Current balance in cents
- `status_count` (map<string,int32>): Counts by rental status (ACTIVE, PENDING, etc.)

## Multi-Org Semantics (FR-004)

| Parameter | Value | Behavior |
|-----------|-------|----------|
| `organization_id` | >0 | Single-org query (existing behavior) |
| `organization_id` | 0 | **Cross-org rollup**: aggregate across all orgs where user has `ACTIVE` membership in `users_orgs` |

**No proto changes required** — sentinel `0` uses existing `int32` field.

## Generated Code
- Go: `api/gen/v1/ledger_service.pb.go` (via `make proto`)
- **NEVER hand-edit generated files** — change proto, then regenerate