package domain

import "time"

type TransactionType string

const (
	TransactionTypeRentalDebit   TransactionType = "RENTAL_DEBIT"
	TransactionTypeLendingCredit TransactionType = "LENDING_CREDIT"
	TransactionTypeRefund        TransactionType = "REFUND"
	TransactionTypeAdjustment    TransactionType = "ADJUSTMENT"
)

type LedgerTransaction struct {
	ID              int32           `json:"id"`
	OrgID           int32           `json:"org_id"`
	UserID          int32           `json:"user_id"`
	Amount          int32           `json:"amount"` // positive for credit, negative for debit
	Type            TransactionType `json:"type"`
	RelatedRentalID *int32          `json:"related_rental_id,omitempty"`
	Description     string          `json:"description"`
	ChargedOn       time.Time       `json:"charged_on"`
	CreatedOn       time.Time       `json:"created_on"`
}
