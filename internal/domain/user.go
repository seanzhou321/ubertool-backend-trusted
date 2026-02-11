package domain

type User struct {
	ID           int32          `json:"id"`
	Email        string         `json:"email"`
	PhoneNumber  string         `json:"phone_number"`
	PasswordHash string         `json:"-"`
	Name         string         `json:"name"`
	AvatarURL    string         `json:"avatar_url"`
	Orgs         []Organization `json:"orgs,omitempty"` // Populated when needed
	CreatedOn    string         `json:"created_on"`
	UpdatedOn    string         `json:"updated_on"`
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
	JoinedOn            string        `json:"joined_on"`
	BalanceCents        int32         `json:"balance_cents"`
	LastBalanceUpdateOn *string       `json:"last_balance_updated_on"`
	Status              UserOrgStatus `json:"status"`
	Role                UserOrgRole   `json:"role"`
	BlockedOn           *string       `json:"blocked_on"`
	BlockedReason       string        `json:"blocked_reason"`
	RentingBlocked      bool          `json:"renting_blocked"`
	LendingBlocked      bool          `json:"lending_blocked"`
	BlockedDueToBillID  *int32        `json:"blocked_due_to_bill_id"`
}
