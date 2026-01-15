package domain

import "time"

type RentalStatus string

const (
	RentalStatusPending   RentalStatus = "PENDING"
	RentalStatusApproved  RentalStatus = "APPROVED"
	RentalStatusScheduled RentalStatus = "SCHEDULED"
	RentalStatusActive    RentalStatus = "ACTIVE"
	RentalStatusCompleted RentalStatus = "COMPLETED"
	RentalStatusCancelled RentalStatus = "CANCELLED"
	RentalStatusOverdue   RentalStatus = "OVERDUE"
)

type Rental struct {
	ID               int32        `json:"id"`
	OrgID            int32        `json:"org_id"`
	ToolID           int32        `json:"tool_id"`
	RenterID         int32        `json:"renter_id"`
	OwnerID          int32        `json:"owner_id"`
	StartDate        time.Time    `json:"start_date"`
	ScheduledEndDate time.Time    `json:"scheduled_end_date"`
	EndDate          *time.Time   `json:"end_date,omitempty"`
	TotalCostCents   int32        `json:"total_cost_cents"`
	Status           RentalStatus `json:"status"`
	PickupNote       string       `json:"pickup_note"`
	CreatedOn        time.Time    `json:"created_on"`
	UpdatedOn        time.Time    `json:"updated_on"`
}
