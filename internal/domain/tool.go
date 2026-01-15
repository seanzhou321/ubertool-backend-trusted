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
	ToolConditionExcellent ToolCondition = "EXCELLENT"
	ToolConditionGood      ToolCondition = "GOOD"
	ToolConditionAcceptable ToolCondition = "ACCEPTABLE"
	ToolConditionDamaged   ToolCondition = "DAMAGED/NEEDS_REPAIR"
)

type Tool struct {
	ID                   int32         `json:"id"`
	OwnerID              int32         `json:"owner_id"`
	Name                 string        `json:"name"`
	Description          string        `json:"description"`
	Categories           []string      `json:"categories"`
	PricePerDayCents     int32         `json:"price_per_day_cents"`
	PricePerWeekCents    int32         `json:"price_per_week_cents"`
	PricePerMonthCents   int32         `json:"price_per_month_cents"`
	ReplacementCostCents int32         `json:"replacement_cost_cents"`
	Condition            ToolCondition `json:"condition"`
	Metro                string        `json:"metro"`
	Status               ToolStatus    `json:"status"`
	CreatedOn            time.Time     `json:"created_on"`
	DeletedOn            *time.Time    `json:"deleted_on,omitempty"`
}

type ToolImage struct {
	ID           int32  `json:"id"`
	ToolID       int32  `json:"tool_id"`
	ImageURL     string `json:"image_url"`
	DisplayOrder int32  `json:"display_order"`
}
