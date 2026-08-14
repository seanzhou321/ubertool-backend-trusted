# Bill Split Feature — Design Gap Analysis

**Date**: 2026-08-13  
**Feature**: 008-bill-split (As-Built Retrofit)  
**Source**: spec.md + constitution.md + checklist + RTM

---

## Executive Summary

The as-built spec is thorough for functional behavior (5 user stories, 16 FRs, detailed acceptance scenarios). However, as a retrofit spec describing *already-deployed* behavior, it leaves several **architectural and operational concerns implicit or absent** — areas where the current code may have implicit behavior that isn't documented, tested, or intentionally designed.

This analysis identifies 8 design gaps across 4 categories, prioritized by **impact × uncertainty** (i.e., likelihood of production incidents or rework if unaddressed).

---

## Gap Taxonomy & Prioritization

| # | Category | Gap | Impact | Uncertainty | Priority |
|---|----------|-----|--------|-------------|----------|
| 1 | Concurrency | Race conditions on bill state transitions | Critical | High | **P0** |
| 2 | Observability | No logging/metrics/tracing requirements | High | High | **P0** |
| 3 | Reliability | Email delivery failure modes & retry policy | High | Medium | **P1** |
| 4 | Scalability | Volume/scale assumptions undocumented | Medium | High | **P1** |
| 5 | Security | Threat model & rate limiting absent | Medium | Medium | **P2** |
| 6 | Compliance | Financial regulatory constraints not addressed | Medium | Low | **P2** |
| 7 | UX/Accessibility | Error states, localization, a11y not specified | Low | Medium | **P3** |
| 8 | Data Model | `ADMIN_COMMENT` modeled but unused (Known Discrepancy 3) | Low | Low | **P3** |

---

## Detailed Gap Analysis

### 1. Concurrency & Race Conditions (P0 — Critical)

**Current State**: 
- `ON CONFLICT DO NOTHING` handles duplicate bill creation (US1 Scenario 3)
- Application-level status checks guard `AcknowledgePayment` and `ResolveDispute` (e.g., "reject if not PENDING/DISPUTED")
- No explicit locking, version columns, or transaction isolation documented

**Scenarios Not Addressed**:
| Scenario | Risk |
|----------|------|
| `CheckOverdueBills` (10-day auto-dispute) runs while debtor calls `AcknowledgePayment` | Bill could transition to `DISPUTED` *after* debtor ack but *before* creditor ack, or debtor ack lost |
| `ResolveDisputedBills` (month-end) runs while admin calls `ResolveDispute` | Double-resolution: admin resolves gracefully, then system auto-resolves as `BOTH_FAULT` with blocking |
| Two admins call `ResolveDispute` on same bill simultaneously | Last-write-wins; one admin's resolution silently overwritten |
| `SendBillSplittingNotices` retries debtor email while bill is being acknowledged | `notice_sent_at` stamp race with acknowledgment logic |
| Concurrent `AcknowledgePayment` (debtor + creditor at same time) | Both may pass "other party hasn't acknowledged" check |

**Recommended Approach**: **Pessimistic locking for background jobs, optimistic for user RPCs**
- Add `version INTEGER NOT NULL DEFAULT 1` to `bills` table
- Background jobs (`CheckOverdueBills`, `ResolveDisputedBills`, `SendBillSplittingNotices`, `SendBillReminders`): `SELECT ... FOR UPDATE` at transaction start
- User RPCs (`AcknowledgePayment`, `ResolveDispute`): `UPDATE ... SET ..., version = version + 1 WHERE id = $1 AND version = $2`; return conflict error on 0 rows affected
- Idempotency keys on user-facing RPCs (debtor/creditor acknowledgment) as defense-in-depth

**Constitutional Alignment**: Principle II (typed domain constants for status) + Principle IV (testable at integration tier with SBR-Trace)

---

### 2. Observability Requirements (P0 — Critical)

**Current State**: 
- No logging, metrics, or tracing requirements in spec
- Background jobs run silently; no visibility into success/failure/latency
- No correlation IDs across job → service → repository → DB

**Missing**:
| Signal | Need |
|--------|------|
| Structured logging | Every state transition (bill status, notice sent, reminder sent, dispute opened, resolution) with `bill_id`, `org_id`, `actor`, `old_status`, `new_status`, `duration_ms` |
| Metrics | `bill_split.bills_created_total`, `bill_split.notices_sent_total`, `bill_split.acknowledgments_total`, `bill_split.disputes_opened_total`, `bill_split.resolutions_total{outcome}`, `bill_split.job_duration_seconds{job}` |
| Tracing | OpenTelemetry spans for each RPC and job execution; propagation via gRPC metadata |
| Alerting | Job failure alerting (cronjob exit code �� 0), stuck bills (PENDING > 14 days without notice_sent_at), dispute resolution SLA breach |

**Recommended Approach**: Define observability requirements as **FR-017 through FR-020** in spec; implement as part of next `/speckit-plan` pass. Minimum viable: structured JSON logs + Prometheus metrics on all state transitions.

---

### 3. Email Delivery Failure Modes & Retry Policy (P1 — High)

**Current State**: 
- US1 Scenario 4: `notice_sent_at` stamped **only on debtor email success**; creditor email failure doesn't block
- US3 Scenario 4: Bill with never-stamped `notice_sent_at` never enters overdue pipeline (accepted as current behavior)
- No retry policy, dead-letter handling, or max-attempts documented

**Gaps**:
| Question | Current Behavior (Inferred) | Risk |
|----------|----------------------------|------|
| Max retry attempts for debtor notice? | Unbounded (job re-runs until success) | Bill stuck in PENDING indefinitely; no alert |
| Retry backoff schedule? | Likely fixed cron interval (daily?) | Thundering herd on email provider outage |
| Creditor notice failure handling? | Logged but ignored | Creditor never notified; no retry |
| Dead-letter / manual intervention path? | None | Ops cannot force-notice a stuck bill |
| Idempotency of notice send? | `notice_sent_at` prevents re-send to debtor | Creditor may get duplicate notices on retry |

**Recommended Approach**: 
- Add `notice_attempts INTEGER DEFAULT 0`, `last_notice_attempt_at TIMESTAMPTZ` to `bills`
- Exponential backoff: 1h, 4h, 12h, 24h, then daily (max 14 attempts = 2 weeks)
- After max attempts: set `notice_failed = true`, alert ops, allow admin "force notice" RPC
- Creditor notice: same retry logic, independent of debtor; `creditor_notice_sent_at` column
- Document in spec as FR-021, FR-022

---

### 4. Volume & Scale Assumptions (P1 — High)

**Current State**: No scale targets documented. The netting algorithm (`CalculateTransactions`) is O(n²) in worst case (all members have balances with all others).

**Missing**:
| Parameter | Need |
|-----------|------|
| Max org members | Affects netting complexity, bill count, notification fan-out |
| Max bills per org per month | Affects `ListPayments` unpaginated response size (Known Discrepancy 1) |
| Concurrent settlement jobs | Multiple orgs settling simultaneously — DB connection pool, lock contention |
| Balance snapshot retention | How many months of `balance_snapshots` retained? |
| Email volume per settlement run | Debtor + creditor per bill × orgs × members — SES/sendgrid limits |

**Recommended Approach**: Document assumptions as **Assumptions** section in spec:
- Target: ≤ 500 members/org, ≤ 1000 bills/org/month, ≤ 50 orgs settling concurrently
- Pagination implementation (FR-013) becomes mandatory if exceeded
- Add integration test with 500-member org to validate performance

---

### 5. Security: Threat Model & Rate Limiting (P2 — Medium)

**Current State**: 
- Role-based access (ADMIN/SUPER_ADMIN, debtor, creditor) enforced in service layer
- No rate limiting on any RPC
- No audit logging of authorization decisions (allow/deny)
- No mention of PII in bill data (amounts, user IDs, org IDs)

**Gaps**:
| Threat | Mitigation Needed |
|--------|-------------------|
| Credential stuffing / brute force on `AcknowledgePayment` | Rate limit per user/IP; lockout after N failures |
| Admin enumeration via `ListDisputedPayments` | Rate limit; audit log every admin list call |
| Bill amount enumeration via `GetPaymentDetail` | Authorization already restricts to parties + admin; add audit log |
| Data exfiltration via global summary | Already aggregated counts only; low risk |
| SQL injection | Parameterized queries already used; document as assumption |

**Recommended Approach**: 
- Add `rate_limit_tier` config per RPC (strict for mutating, lenient for reads)
- Structured audit log: `actor`, `rpc`, `bill_id`, `allowed`, `reason` → append-only table
- Document as FR-023, FR-024

---

### 6. Compliance & Regulatory (P2 — Medium)

**Current State**: Financial transaction system (debt settlement, balance transfers, blocking) with no regulatory analysis.

**Potential Obligations** (jurisdiction-dependent):
| Area | Question |
|------|----------|
| Money transmission | Does bill-split constitute money transmission requiring license? |
| Consumer protection | Dispute resolution timelines, notice requirements (US3: 10-day auto-dispute) |
| Data retention | How long must bill records be kept? (GDPR, SOX, local financial regs) |
| Right to erasure | Can a user request bill history deletion? Blocks `bill_actions` audit trail |
| Audit trail | `bill_actions` is immutable — good; but is it tamper-evident? |

**Recommended Approach**: 
- Add **Compliance** subsection to Assumptions: "This spec assumes no regulated financial activity; if jurisdiction changes, a compliance review is required before deployment."
- Document data retention policy (e.g., 7 years for financial records) as FR-025

---

### 7. UX: Error States, Accessibility, Localization (P3 — Low)

**Current State**: 
- gRPC error codes used (implied by "rejected with an error" in scenarios)
- No user-facing error message catalog
- No localization strategy (email templates, in-app notifications)
- No accessibility requirements for dashboard counts

**Gaps**: Low priority for backend-only spec; relevant when frontend consumes these RPCs.

**Recommended Approach**: Defer to frontend spec; add note in Assumptions: "Error message localization and a11y are frontend concerns; backend returns standardized gRPC status codes only."

---

### 8. Unused `ADMIN_COMMENT` Capability (P3 — Low)

**Current State**: Known Discrepancy 3 — `domain.BillActionTypeAdminComment` and `bill_actions.action_details` (JSONB) exist but no API.

**Options**:
| Option | Pros | Cons |
|--------|------|------|
| Remove from domain/schema | Cleanup; reduces confusion | Migration needed; may break if any code references it |
| Implement `AddDisputeComment` RPC | Completes modeled capability; useful for admin audit trail | Scope creep for retrofit spec |
| Document as "reserved for future use" | Zero code change; honest | Dead code accumulates |

**Recommended Approach**: Document as "reserved" in spec; create follow-up task to either implement or remove in next feature cycle. Not a clarification blocker.

---

## Recommended Clarification Questions (for `/speckit-clarify`)

If running the interactive clarification, these 5 questions would be asked (in priority order):

1. **Concurrency model** — Pessimistic locking for jobs + optimistic for RPCs (Option C)?
2. **Observability requirements** — Structured logs + Prometheus metrics + OpenTelemetry tracing on all state transitions?
3. **Email retry policy** — Exponential backoff with max attempts, dead-letter alerting, independent creditor retries?
4. **Scale targets** — Document assumed max org size, bill volume, concurrency; make pagination mandatory if exceeded?
5. **Rate limiting & audit logging** — Per-RPC rate limits + authorization audit log table?

---

## Suggested Spec Updates

| FR ID | Description | Section |
|-------|-------------|---------|
| FR-017 | Structured logging on every bill state transition | Non-Functional → Observability |
| FR-018 | Prometheus metrics for bill lifecycle counters | Non-Functional → Observability |
| FR-019 | OpenTelemetry tracing for all RPCs and jobs | Non-Functional → Observability |
| FR-020 | Alerting rules for job failures and stuck bills | Non-Functional → Observability |
| FR-021 | Email retry policy with exponential backoff | Integration → External Dependencies |
| FR-022 | Dead-letter handling & admin force-notice capability | Integration → External Dependencies |
| FR-023 | Per-RPC rate limiting configuration | Non-Functional → Security |
| FR-024 | Authorization audit log (append-only) | Non-Functional → Security |
| FR-025 | Data retention policy for financial records | Assumptions → Compliance |

---

## Next Steps

1. **Run `/speckit-clarify`** with this analysis as context — it will ask the top 5 questions interactively and write answers to `spec.md` under `## Clarifications`
2. **Run `/speckit-plan`** — will generate `plan.md` incorporating clarified requirements
3. **Run `/speckit-tasks`** — will produce `tasks.md` with implementation tasks for gaps (especially concurrency, observability, email retry)
4. **Consider `/speckit-sbr-audit`** — current RTM shows gaps in `CheckOverdueBills`, `ResolveDisputedBills`, graceful-dispute-acknowledgment; these align with concurrency/observability gaps

---

## Files Referenced

- `specs/008-bill-split/spec.md` — As-built feature specification
- `.specify/memory/constitution.md` — Project constitution v2.0.0
- `specs/008-bill-split/checklists/requirements.md` — Quality checklist
- `sbr/rtm/008-bill-split.rtm.md` — Requirements Traceability Matrix
- `docs/design/grpc_api_business_logic.md` — Business logic reference (lower trust)
- `docs/improvements/bill-split-api-review.md` — Stale improvement doc (superseded)