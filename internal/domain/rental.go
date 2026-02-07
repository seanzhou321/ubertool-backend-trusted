package domain

import "time"

type RentalStatus string

const (
	RentalStatusPending                  RentalStatus = "PENDING"
	RentalStatusApproved                 RentalStatus = "APPROVED"
	RentalStatusRejected                 RentalStatus = "REJECTED"
	RentalStatusScheduled                RentalStatus = "SCHEDULED"
	RentalStatusActive                   RentalStatus = "ACTIVE"
	RentalStatusCompleted                RentalStatus = "COMPLETED"
	RentalStatusCancelled                RentalStatus = "CANCELLED"
	RentalStatusOverdue                  RentalStatus = "OVERDUE"
	RentalStatusReturnDateChanged        RentalStatus = "RETURN_DATE_CHANGED"
	RentalStatusReturnDateChangeRejected RentalStatus = "RETURN_DATE_CHANGE_REJECTED"
)

type Rental struct {
	ID                     int32        `json:"id"`
	OrgID                  int32        `json:"org_id"`
	ToolID                 int32        `json:"tool_id"`
	RenterID               int32        `json:"renter_id"`
	OwnerID                int32        `json:"owner_id"`
	StartDate              time.Time    `json:"start_date"`
	EndDate                time.Time    `json:"end_date"`
	LastAgreedEndDate      *time.Time   `json:"last_agreed_end_date,omitempty"`
	TotalCostCents         int32        `json:"total_cost_cents"`
	Status                 RentalStatus `json:"status"`
	CompletedBy            *int32       `json:"completed_by,omitempty"`
	PickupNote             string       `json:"pickup_note"`
	RejectionReason        string       `json:"rejection_reason"`
	ReturnCondition        string       `json:"return_condition"`
	SurchargeOrCreditCents int32        `json:"surcharge_or_credit_cents"`
	CreatedOn              time.Time    `json:"created_on"`
	UpdatedOn              time.Time    `json:"updated_on"`
}
