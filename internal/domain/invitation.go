package domain

type Invitation struct {
	ID             int32   `json:"id"`
	InvitationCode string  `json:"invitation_code"`
	OrgID          int32   `json:"org_id"`
	Email          string  `json:"email"`
	CreatedBy      int32   `json:"created_by"`
	ExpiresOn      string  `json:"expires_on"`
	UsedOn         *string `json:"used_on,omitempty"`
	UsedByUserID   *int32  `json:"used_by_user_id,omitempty"`
	CreatedOn      string  `json:"created_on"`
}
