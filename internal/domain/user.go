package domain

import "time"

type User struct {
	ID           int32          `json:"id"`
	Email        string         `json:"email"`
	PhoneNumber  string         `json:"phone_number"`
	PasswordHash string         `json:"-"`
	Name         string         `json:"name"`
	AvatarURL    string         `json:"avatar_url"`
	Orgs         []Organization `json:"orgs,omitempty"` // Populated when needed
	CreatedOn    time.Time      `json:"created_on"`
	UpdatedOn    time.Time      `json:"updated_on"`
}

type UserOrgStatus string

const (
	UserOrgStatusActive  UserOrgStatus = "ACTIVE"
	UserOrgStatusSuspend UserOrgStatus = "SUSPEND"
	UserOrgStatusBlock   UserOrgStatus = "BLOCK"
)

type UserOrgRole string

const (
	UserOrgRoleSuperAdmin UserOrgRole = "SUPER_ADMIN"
	UserOrgRoleAdmin      UserOrgRole = "ADMIN"
	UserOrgRoleMember     UserOrgRole = "MEMBER"
)

type UserOrg struct {
	UserID              int32         `json:"user_id"`
	OrgID               int32         `json:"org_id"`
	JoinedOn            time.Time     `json:"joined_on"`
	BalanceCents        int32         `json:"balance_cents"`
	LastBalanceUpdateOn *time.Time    `json:"last_balance_updated_on"`
	Status              UserOrgStatus `json:"status"`
	Role                UserOrgRole   `json:"role"`
	BlockedDate         *time.Time    `json:"blocked_date"`
	BlockReason         string        `json:"block_reason"`
	RentingBlocked      bool          `json:"renting_blocked"`
	LendingBlocked      bool          `json:"lending_blocked"`
	BlockedDueToBillID  *int32        `json:"blocked_due_to_bill_id"`
	BillBlockReason     string        `json:"bill_block_reason"`
}
