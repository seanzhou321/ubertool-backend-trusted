package domain

type TransactionType string

const (
	TransactionTypeRentalDebit   TransactionType = "RENTAL_DEBIT"
	TransactionTypeLendingCredit TransactionType = "LENDING_CREDIT"
	TransactionTypeLendingDebit  TransactionType = "LENDING_DEBIT"
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
	ChargedOn       string          `json:"charged_on"`
	CreatedOn       string          `json:"created_on"`
}

type LedgerSummary struct {
	Balance              int32            `json:"balance"`
	ActiveRentalsCount   int32            `json:"active_rentals_count"`
	ActiveLendingsCount  int32            `json:"active_lendings_count"`
	PendingRequestsCount int32            `json:"pending_requests_count"`
	StatusCount          map[string]int32 `json:"status_count"`
}
