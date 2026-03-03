package domain

type Organization struct {
	ID                              int32  `json:"id"`
	Name                            string `json:"name"`
	Description                     string `json:"description"`
	Address                         string `json:"address"`
	Metro                           string `json:"metro"`
	AdminPhoneNumber                string `json:"admin_phone_number"`
	AdminEmail                      string `json:"admin_email"`
	CreatedOn                       string `json:"created_on"`
	MemberCount                     int32  `json:"member_count"`                        // Count of non-blocked members
	Admins                          []User `json:"admins,omitempty"`                    // List of SUPER_ADMIN and ADMIN users, populated in SearchOrganizations
	SettlementThresholdCents        int32  `json:"settlement_threshold_cents"`         // Max amount allowed to carry over after bill splitting
	MaxBillsplitRentalCostCents     int32  `json:"max_billsplit_rental_cost_cents"`    // Max rental cost settled by bill splitting
}
