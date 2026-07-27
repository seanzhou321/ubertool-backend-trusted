# Bill Split Contracts

Source: `api/proto/ubertool_trusted_backend/v1/bill_split_service.proto`

## BillSplitService RPCs

| RPC | Request | Response | Auth |
|-----|---------|----------|------|
| `ListPayments` | `ListPaymentsRequest { org_id, status?, page }` | `ListPaymentsResponse { repeated Bill }` | Org member |
| `GetPaymentDetail` | `GetPaymentDetailRequest { bill_id }` | `GetPaymentDetailResponse { Bill }` | Debtor, creditor, or org admin |
| `AcknowledgePayment` | `AcknowledgePaymentRequest { bill_id }` | `AcknowledgePaymentResponse { Bill }` | Debtor or creditor |
| `ResolveDispute` | `ResolveDisputeRequest { bill_id, resolution }` | `ResolveDisputeResponse { Bill }` | **Org ADMIN/SUPER_ADMIN** |
| `ListDisputedPayments` | `ListDisputedPaymentsRequest { org_id }` | `ListDisputedPaymentsResponse { repeated Bill }` | Org admin |
| `ListResolvedDisputes` | `ListResolvedDisputesRequest { org_id }` | `ListResolvedDisputesResponse { repeated Bill }` | Org admin |
| **NEW FR-014** `GetGlobalBillSplitSummary` | `GetGlobalBillSplitSummaryRequest {}` | `GetGlobalBillSplitSummaryResponse` | Any authenticated user |
| **NEW FR-015** `GetOrganizationBillSplitSummary` | `GetOrganizationBillSplitSummaryRequest { organization_id }` | `GetOrganizationBillSplitSummaryResponse` | **Org ADMIN/SUPER_ADMIN** |

## Bill Message

```protobuf
message Bill {
  int64 id = 1;
  string organization_id = 2;
  string debtor_id = 3;
  string creditor_id = 4;
  int64 amount_cents = 5;
  string status = 6;           // PENDING, ACKNOWLEDGED, DISPUTED, RESOLVED, EXPIRED
  string settlement_month = 7; // YYYY-MM
  string notice_sent_at = 8;
  string acknowledged_at = 9;
  string disputed_at = 10;
  string resolved_at = 11;
  string created_at = 12;
  string updated_at = 13;
}
```

## FR-014: Global Summary Response

```protobuf
message GetGlobalBillSplitSummaryResponse {
  int32 pending_count = 1;
  int32 acknowledged_count = 2;
  int32 disputed_count = 3;
  int32 resolved_count = 4;
  int64 total_amount_cents = 5;  // Sum of non-resolved bills
}
```

**Scope**: All orgs where user has ACTIVE membership
**Filter**: Bills where user is debtor OR creditor
**Excludes**: RESOLVED/EXPIRED from amount

## FR-015: Per-Org Summary Response

```protobuf
message GetOrganizationBillSplitSummaryRequest {
  string organization_id = 1;
}

message GetOrganizationBillSplitSummaryResponse {
  int32 pending_count = 1;
  int32 acknowledged_count = 2;
  int32 disputed_count = 3;
  int32 resolved_count = 4;
  int64 total_amount_cents = 5;
}
```

**Scope**: Single organization
**Auth**: Caller must be ADMIN or SUPER_ADMIN in target org
**Filter**: Bills in that org where caller is debtor OR creditor (admin view = all bills in org)

## Authorization Matrix

| RPC | Required Role | Org Scope |
|-----|---------------|-----------|
| `ListPayments` | Member | Current org |
| `GetPaymentDetail` | Debtor/Creditor/Admin | Bill's org |
| `AcknowledgePayment` | Debtor/Creditor | Bill's org |
| `ResolveDispute` | **Admin/SuperAdmin** | Bill's org |
| `ListDisputedPayments` | **Admin/SuperAdmin** | Current org |
| `ListResolvedDisputes` | **Admin/SuperAdmin** | Current org |
| `GetGlobalBillSplitSummary` | Member | **All active orgs** |
| `GetOrganizationBillSplitSummary` | **Admin/SuperAdmin** | Specified org |

## No Breaking Changes

All new RPCs are additive. Existing clients unaffected.