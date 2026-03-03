package domain

import "time"

type Notification struct {
	ID          int64             `json:"id"`
	UserID      int32             `json:"user_id"`
	OrgID       int32             `json:"org_id"`
	Title       string            `json:"title"`
	Message     string            `json:"message"`
	DeliveredAt *time.Time        `json:"delivered_at"`
	ClickedAt   *time.Time        `json:"clicked_at"`
	ReadAt      *time.Time        `json:"read_at"`
	Attributes  map[string]string `json:"attributes"`
	CreatedAt   *time.Time        `json:"created_at"`
}
