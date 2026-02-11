package domain

type Notification struct {
	ID         int32             `json:"id"`
	UserID     int32             `json:"user_id"`
	OrgID      int32             `json:"org_id"`
	Title      string            `json:"title"`
	Message    string            `json:"message"`
	IsRead     bool              `json:"is_read"`
	Attributes map[string]string `json:"attributes"`
	CreatedOn  string            `json:"created_on"`
}
