package domain

import "time"

type ToolStatus string

const (
	ToolStatusAvailable   ToolStatus = "AVAILABLE"
	ToolStatusUnavailable ToolStatus = "UNAVAILABLE"
	ToolStatusRented      ToolStatus = "RENTED"
)

type ToolCondition string

const (
	ToolConditionExcellent  ToolCondition = "EXCELLENT"
	ToolConditionGood       ToolCondition = "GOOD"
	ToolConditionAcceptable ToolCondition = "ACCEPTABLE"
	ToolConditionDamaged    ToolCondition = "DAMAGED/NEEDS_REPAIR"
)

type ToolDurationUnit string

const (
	ToolDurationUnitDay   ToolDurationUnit = "day"
	ToolDurationUnitWeek  ToolDurationUnit = "week"
	ToolDurationUnitMonth ToolDurationUnit = "month"
)

type Tool struct {
	ID                   int32            `json:"id"`
	OwnerID              int32            `json:"owner_id"`
	Owner                *User            `json:"owner,omitempty"` // Populated when fetching tool details
	Name                 string           `json:"name"`
	Description          string           `json:"description"`
	Categories           []string         `json:"categories"`
	PricePerDayCents     int32            `json:"price_per_day_cents"`
	PricePerWeekCents    int32            `json:"price_per_week_cents"`
	PricePerMonthCents   int32            `json:"price_per_month_cents"`
	ReplacementCostCents int32            `json:"replacement_cost_cents"`
	DurationUnit         ToolDurationUnit `json:"duration_unit"`
	Condition            ToolCondition    `json:"condition"`
	Metro                string           `json:"metro"`
	Status               ToolStatus       `json:"status"`
	CreatedOn            string           `json:"created_on"`
	DeletedOn            *string          `json:"deleted_on,omitempty"`
}

type ToolImage struct {
	ID            int32      `json:"id"`
	ToolID        int32      `json:"tool_id"`
	UserID        int32      `json:"user_id"`
	FileName      string     `json:"file_name"`
	FilePath      string     `json:"file_path"`
	ThumbnailPath string     `json:"thumbnail_path"`
	FileSize      int64      `json:"file_size"`
	MimeType      string     `json:"mime_type"`
	IsPrimary     bool       `json:"is_primary"`
	DisplayOrder  int32      `json:"display_order"`
	Status        string     `json:"status"`               // PENDING, CONFIRMED, DELETED
	ExpiresAt     *time.Time `json:"expires_at,omitempty"` // For pending images
	CreatedOn     time.Time  `json:"created_on"`
	ConfirmedOn   *time.Time `json:"confirmed_on,omitempty"`
	DeletedOn     *time.Time `json:"deleted_on,omitempty"`
}
