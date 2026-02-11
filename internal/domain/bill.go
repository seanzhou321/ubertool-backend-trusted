package domain

import "time"

type BillStatus string

const (
	BillStatusPending             BillStatus = "PENDING"
	BillStatusPaid                BillStatus = "PAID"
	BillStatusDisputed            BillStatus = "DISPUTED"
	BillStatusAdminResolved       BillStatus = "ADMIN_RESOLVED"
	BillStatusSystemDefaultAction BillStatus = "SYSTEM_DEFAULT_ACTION"
)

type DisputeReason string

const (
	DisputeReasonDebtorNoAck   DisputeReason = "DEBTOR_NO_ACK"
	DisputeReasonCreditorNoAck DisputeReason = "CREDITOR_NO_ACK"
)

type ResolutionOutcome string

const (
	ResolutionOutcomeGraceful      ResolutionOutcome = "GRACEFUL"
	ResolutionOutcomeDebtorFault   ResolutionOutcome = "DEBTOR_FAULT"
	ResolutionOutcomeCreditorFault ResolutionOutcome = "CREDITOR_FAULT"
	ResolutionOutcomeBothFault     ResolutionOutcome = "BOTH_FAULT"
)

type Bill struct {
	ID                     int32      `json:"id"`
	OrgID                  int32      `json:"org_id"`
	DebtorUserID           int32      `json:"debtor_user_id"`
	CreditorUserID         int32      `json:"creditor_user_id"`
	AmountCents            int32      `json:"amount_cents"`
	SettlementMonth        string     `json:"settlement_month"` // Format: 'YYYY-MM'
	Status                 BillStatus `json:"status"`
	NoticeSentAt           *time.Time `json:"notice_sent_at"`
	DebtorAcknowledgedAt   *time.Time `json:"debtor_acknowledged_at"`
	CreditorAcknowledgedAt *time.Time `json:"creditor_acknowledged_at"`
	DisputedAt             *time.Time `json:"disputed_at"`
	ResolvedAt             *time.Time `json:"resolved_at"`
	DisputeReason          string     `json:"dispute_reason"`
	ResolutionOutcome      string     `json:"resolution_outcome"`
	ResolutionNotes        string     `json:"resolution_notes"`
	CreatedAt              time.Time  `json:"created_at"`
	UpdatedAt              time.Time  `json:"updated_at"`
}

// Helper to determine payment category for UI
func (b *Bill) GetPaymentCategory(userID int32) string {
	isDebtor := b.DebtorUserID == userID
	isCreditor := b.CreditorUserID == userID

	switch b.Status {
	case BillStatusPending:
		if isDebtor && b.DebtorAcknowledgedAt == nil {
			return "PAYMENT_TO_MAKE"
		}
		if isCreditor && b.DebtorAcknowledgedAt != nil {
			return "RECEIPT_TO_VERIFY"
		}
	case BillStatusDisputed:
		if isDebtor {
			return "PAYMENT_IN_DISPUTE"
		}
		if isCreditor {
			return "RECEIPT_IN_DISPUTE"
		}
	case BillStatusPaid, BillStatusAdminResolved, BillStatusSystemDefaultAction:
		return "COMPLETED"
	}
	return "COMPLETED"
}

type BillActionType string

const (
	BillActionTypeNoticeSent           BillActionType = "NOTICE_SENT"
	BillActionTypeDebtorAcknowledged   BillActionType = "DEBTOR_ACKNOWLEDGED"
	BillActionTypeCreditorAcknowledged BillActionType = "CREDITOR_ACKNOWLEDGED"
	BillActionTypeDisputeOpened        BillActionType = "DISPUTE_OPENED"
	BillActionTypeAdminComment         BillActionType = "ADMIN_COMMENT"
	BillActionTypeAdminResolution      BillActionType = "ADMIN_RESOLUTION"
	BillActionTypeSystemAutoResolve    BillActionType = "SYSTEM_AUTO_RESOLVE"
)

type BillAction struct {
	ID            int32          `json:"id"`
	BillID        int32          `json:"bill_id"`
	ActorUserID   *int32         `json:"actor_user_id"` // NULL for system actions
	ActionType    BillActionType `json:"action_type"`
	ActionDetails string         `json:"action_details"` // JSONB stored as string
	Notes         string         `json:"notes"`
	CreatedAt     time.Time      `json:"created_at"`
}
