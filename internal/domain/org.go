package domain

import "time"

type Organization struct {
	ID               int32     `json:"id"`
	Name             string    `json:"name"`
	Description      string    `json:"description"`
	Address          string    `json:"address"`
	Metro            string    `json:"metro"`
	AdminPhoneNumber string    `json:"admin_phone_number"`
	AdminEmail       string    `json:"admin_email"`
	CreatedOn        time.Time `json:"created_on"`
	Admins           []User    `json:"admins,omitempty"` // List of SUPER_ADMIN and ADMIN users, populated in SearchOrganizations
}
