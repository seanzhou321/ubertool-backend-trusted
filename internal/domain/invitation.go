package domain

import (
	"time"
)

type Invitation struct {
	Token     string     `json:"token"` // UUID
	OrgID     int32      `json:"org_id"`
	Email     string     `json:"email"`
	CreatedBy int32      `json:"created_by"`
	ExpiresOn time.Time  `json:"expires_on"`
	UsedOn    *time.Time `json:"used_on,omitempty"`
	CreatedOn time.Time  `json:"created_on"`
}
