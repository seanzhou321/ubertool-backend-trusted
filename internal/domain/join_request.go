package domain

type JoinRequestStatus string

const (
	JoinRequestStatusPending  JoinRequestStatus = "PENDING"
	JoinRequestStatusApproved JoinRequestStatus = "APPROVED"
	JoinRequestStatusRejected JoinRequestStatus = "REJECTED"
)

type JoinRequest struct {
	ID        int32             `json:"id"`
	OrgID     int32             `json:"org_id"`
	UserID    *int32            `json:"user_id"`
	Name      string            `json:"name"`
	Email     string            `json:"email"`
	Note      string            `json:"note"`
	Status    JoinRequestStatus `json:"status"`
	CreatedOn string            `json:"created_on"`
	UsedOn    *string           `json:"used_on,omitempty"` // Date when invitation code was used
}
