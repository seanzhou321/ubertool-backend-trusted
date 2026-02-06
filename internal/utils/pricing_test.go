package utils

import (
	"testing"
	"time"

	"ubertool-backend-trusted/internal/domain"

	"github.com/stretchr/testify/assert"
)

func TestParseDate(t *testing.T) {
	t.Run("Valid date", func(t *testing.T) {
		date, err := ParseDate("2024-01-15")
		assert.NoError(t, err)
		assert.Equal(t, 2024, date.Year)
		assert.Equal(t, 1, date.Month)
		assert.Equal(t, 15, date.Day)
	})

	t.Run("Invalid format", func(t *testing.T) {
		_, err := ParseDate("2024/01/15")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid date format")
	})

	t.Run("Invalid month", func(t *testing.T) {
		_, err := ParseDate("2024-13-15")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "month must be between 1 and 12")
	})

	t.Run("Invalid day", func(t *testing.T) {
		_, err := ParseDate("2024-01-32")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "day must be between 1 and 31")
	})
}

func TestDaysInMonth(t *testing.T) {
	tests := []struct {
		year     int
		month    int
		expected int
	}{
		{2024, 1, 31},  // January
		{2024, 2, 29},  // February (leap year)
		{2023, 2, 28},  // February (non-leap year)
		{2024, 4, 30},  // April
		{2024, 6, 30},  // June
		{2024, 9, 30},  // September
		{2024, 11, 30}, // November
		{2024, 12, 31}, // December
		{2000, 2, 29},  // Leap year (divisible by 400)
		{1900, 2, 28},  // Not a leap year (divisible by 100 but not 400)
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			days := DaysInMonth(tt.year, tt.month)
			assert.Equal(t, tt.expected, days)
		})
	}
}

func TestCalculateDateDifference(t *testing.T) {
	t.Run("Same day", func(t *testing.T) {
		start := Date{Year: 2024, Month: 1, Day: 15}
		end := Date{Year: 2024, Month: 1, Day: 15}
		diff, err := CalculateDateDifference(start, end)
		assert.NoError(t, err)
		assert.Equal(t, 0, diff.Months)
		assert.Equal(t, 1, diff.Days) // Including both ends
	})

	t.Run("Same month", func(t *testing.T) {
		start := Date{Year: 2024, Month: 1, Day: 15}
		end := Date{Year: 2024, Month: 1, Day: 20}
		diff, err := CalculateDateDifference(start, end)
		assert.NoError(t, err)
		assert.Equal(t, 0, diff.Months)
		assert.Equal(t, 6, diff.Days) // 20 - 15 + 1 = 6
	})

	t.Run("Cross month boundary", func(t *testing.T) {
		start := Date{Year: 2024, Month: 1, Day: 25}
		end := Date{Year: 2024, Month: 2, Day: 5}
		diff, err := CalculateDateDifference(start, end)
		assert.NoError(t, err)
		assert.Equal(t, 0, diff.Months)
		assert.Equal(t, 12, diff.Days) // Jan 25-31 (7 days) + Feb 1-5 (5 days) = 12
	})

	t.Run("Multiple months", func(t *testing.T) {
		start := Date{Year: 2024, Month: 1, Day: 15}
		end := Date{Year: 2024, Month: 4, Day: 20}
		diff, err := CalculateDateDifference(start, end)
		assert.NoError(t, err)
		assert.Equal(t, 3, diff.Months)
		assert.Equal(t, 6, diff.Days) // 20 - 15 + 1 = 6
	})

	t.Run("Exact months", func(t *testing.T) {
		start := Date{Year: 2024, Month: 1, Day: 15}
		end := Date{Year: 2024, Month: 3, Day: 14}
		diff, err := CalculateDateDifference(start, end)
		assert.NoError(t, err)
		assert.Equal(t, 2, diff.Months)
		assert.Equal(t, 0, diff.Days)
	})

	t.Run("Cross year boundary", func(t *testing.T) {
		start := Date{Year: 2023, Month: 11, Day: 15}
		end := Date{Year: 2024, Month: 2, Day: 10}
		diff, err := CalculateDateDifference(start, end)
		assert.NoError(t, err)
		assert.Equal(t, 2, diff.Months)
		assert.Equal(t, 27, diff.Days)
	})

	t.Run("End before start", func(t *testing.T) {
		start := Date{Year: 2024, Month: 1, Day: 20}
		end := Date{Year: 2024, Month: 1, Day: 15}
		_, err := CalculateDateDifference(start, end)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "end date must be >= start date")
	})
}

func TestCalculateRentalCost_DayUnit(t *testing.T) {
	tool := &domain.Tool{
		PricePerDayCents:   1000,  // $10.00
		PricePerWeekCents:  4500,  // $45.00
		PricePerMonthCents: 13500, // $135.00
		DurationUnit:       domain.ToolDurationUnitDay,
	}

	t.Run("Single day", func(t *testing.T) {
		start, _ := time.Parse("2006-01-02", "2024-01-15")
		end, _ := time.Parse("2006-01-02", "2024-01-15")
		cost, err := CalculateRentalCost(start, end, tool)
		assert.NoError(t, err)
		assert.Equal(t, int32(1000), cost) // 1 day
	})

	t.Run("6 days", func(t *testing.T) {
		start, _ := time.Parse("2006-01-02", "2024-01-15")
		end, _ := time.Parse("2006-01-02", "2024-01-20")
		cost, err := CalculateRentalCost(start, end, tool)
		assert.NoError(t, err)
		assert.Equal(t, int32(6000), cost) // 6 days * $10
	})

	t.Run("One week", func(t *testing.T) {
		start, _ := time.Parse("2006-01-02", "2024-01-15")
		end, _ := time.Parse("2006-01-02", "2024-01-21")
		cost, err := CalculateRentalCost(start, end, tool)
		assert.NoError(t, err)
		assert.Equal(t, int32(4500), cost) // 1 week * $45
	})

	t.Run("1 week + 4 days", func(t *testing.T) {
		start, _ := time.Parse("2006-01-02", "2024-01-15")
		end, _ := time.Parse("2006-01-02", "2024-01-25")
		cost, err := CalculateRentalCost(start, end, tool)
		assert.NoError(t, err)
		// 11 days = 1 week (7 days) + 4 days
		assert.Equal(t, int32(8500), cost) // $45 + $40 = $85
	})

	t.Run("2 months + 25 days", func(t *testing.T) {
		start, _ := time.Parse("2006-01-02", "2023-12-15")
		end, _ := time.Parse("2006-01-02", "2024-03-10")
		cost, err := CalculateRentalCost(start, end, tool)
		assert.NoError(t, err)
		// 2 months + 25 days = 2 months + 3 weeks + 4 days
		// $270 + $135 + $40 = $445
		assert.Equal(t, int32(44500), cost)
	})

	t.Run("3 months + 6 days", func(t *testing.T) {
		start, _ := time.Parse("2006-01-02", "2023-12-15")
		end, _ := time.Parse("2006-01-02", "2024-03-20")
		cost, err := CalculateRentalCost(start, end, tool)
		assert.NoError(t, err)
		// 3 months + 6 days = 3 months + 0 weeks + 6 days
		// $405 + $0 + $60 = $465
		assert.Equal(t, int32(46500), cost)
	})
}

func TestCalculateRentalCost_WeekUnit(t *testing.T) {
	tool := &domain.Tool{
		PricePerDayCents:   1000,  // $10.00
		PricePerWeekCents:  4500,  // $45.00
		PricePerMonthCents: 13500, // $135.00
		DurationUnit:       domain.ToolDurationUnitWeek,
	}

	t.Run("Exactly 2 weeks", func(t *testing.T) {
		start, _ := time.Parse("2006-01-02", "2024-01-15")
		end, _ := time.Parse("2006-01-02", "2024-01-28")
		cost, err := CalculateRentalCost(start, end, tool)
		assert.NoError(t, err)
		// 14 days = 2 weeks
		assert.Equal(t, int32(9000), cost) // 2 weeks * $45
	})

	t.Run("10 days rounds up to 2 weeks", func(t *testing.T) {
		start, _ := time.Parse("2006-01-02", "2024-01-15")
		end, _ := time.Parse("2006-01-02", "2024-01-24")
		cost, err := CalculateRentalCost(start, end, tool)
		assert.NoError(t, err)
		// 10 days rounds up to 2 weeks
		assert.Equal(t, int32(9000), cost) // 2 weeks * $45
	})

	t.Run("2 months + 10 days", func(t *testing.T) {
		start, _ := time.Parse("2006-01-02", "2024-01-15")
		end, _ := time.Parse("2006-01-02", "2024-03-24")
		cost, err := CalculateRentalCost(start, end, tool)
		assert.NoError(t, err)
		// 2 months + 10 days = 2 months + 2 weeks (rounded up)
		// $270 + $90 = $360
		assert.Equal(t, int32(36000), cost)
	})
}

func TestCalculateRentalCost_MonthUnit(t *testing.T) {
	tool := &domain.Tool{
		PricePerDayCents:   1000,  // $10.00
		PricePerWeekCents:  4500,  // $45.00
		PricePerMonthCents: 13500, // $135.00
		DurationUnit:       domain.ToolDurationUnitMonth,
	}

	t.Run("Exactly 2 months", func(t *testing.T) {
		start, _ := time.Parse("2006-01-02", "2024-01-15")
		end, _ := time.Parse("2006-01-02", "2024-03-14")
		cost, err := CalculateRentalCost(start, end, tool)
		assert.NoError(t, err)
		// Exactly 2 months (no extra days)
		assert.Equal(t, int32(27000), cost) // 2 months * $135
	})

	t.Run("2 months + 6 days rounds up", func(t *testing.T) {
		start, _ := time.Parse("2006-01-02", "2024-01-15")
		end, _ := time.Parse("2006-01-02", "2024-03-20")
		cost, err := CalculateRentalCost(start, end, tool)
		assert.NoError(t, err)
		// 2 months + 6 days rounds up to 3 months
		assert.Equal(t, int32(40500), cost) // 3 months * $135
	})

	t.Run("3 months + 6 days rounds up", func(t *testing.T) {
		start, _ := time.Parse("2006-01-02", "2023-12-15")
		end, _ := time.Parse("2006-01-02", "2024-03-20")
		cost, err := CalculateRentalCost(start, end, tool)
		assert.NoError(t, err)
		// 3 months + 6 days rounds up to 4 months
		assert.Equal(t, int32(54000), cost) // 4 months * $135
	})

	t.Run("Single day rounds to 1 month", func(t *testing.T) {
		start, _ := time.Parse("2006-01-02", "2024-01-15")
		end, _ := time.Parse("2006-01-02", "2024-01-15")
		cost, err := CalculateRentalCost(start, end, tool)
		assert.NoError(t, err)
		// 1 day rounds up to 1 month
		assert.Equal(t, int32(13500), cost) // 1 month * $135
	})
}

func TestCalculateRentalCostWithBreakdown_DayUnit(t *testing.T) {
	tool := &domain.Tool{
		PricePerDayCents:   1000,  // $10.00
		PricePerWeekCents:  4500,  // $45.00
		PricePerMonthCents: 13500, // $135.00
		DurationUnit:       domain.ToolDurationUnitDay,
	}

	t.Run("2 months + 25 days", func(t *testing.T) {
		start, _ := time.Parse("2006-01-02", "2023-12-15")
		end, _ := time.Parse("2006-01-02", "2024-03-10")
		breakdown, err := CalculateRentalCostWithBreakdown(start, end, tool)
		assert.NoError(t, err)
		assert.Equal(t, 2, breakdown.Months)
		assert.Equal(t, 3, breakdown.Weeks)
		assert.Equal(t, 4, breakdown.Days)
		assert.Equal(t, int32(27000), breakdown.MonthsCost) // 2 * $135
		assert.Equal(t, int32(13500), breakdown.WeeksCost)  // 3 * $45
		assert.Equal(t, int32(4000), breakdown.DaysCost)    // 4 * $10
		assert.Equal(t, int32(44500), breakdown.TotalCost)  // $445
	})

	t.Run("1 week + 4 days", func(t *testing.T) {
		start, _ := time.Parse("2006-01-02", "2024-01-15")
		end, _ := time.Parse("2006-01-02", "2024-01-25")
		breakdown, err := CalculateRentalCostWithBreakdown(start, end, tool)
		assert.NoError(t, err)
		assert.Equal(t, 0, breakdown.Months)
		assert.Equal(t, 1, breakdown.Weeks)
		assert.Equal(t, 4, breakdown.Days)
		assert.Equal(t, int32(0), breakdown.MonthsCost)
		assert.Equal(t, int32(4500), breakdown.WeeksCost) // 1 * $45
		assert.Equal(t, int32(4000), breakdown.DaysCost)  // 4 * $10
		assert.Equal(t, int32(8500), breakdown.TotalCost) // $85
	})
}

func TestCalculateRentalCostWithBreakdown_MonthUnit(t *testing.T) {
	tool := &domain.Tool{
		PricePerDayCents:   1000,  // $10.00
		PricePerWeekCents:  4500,  // $45.00
		PricePerMonthCents: 13500, // $135.00
		DurationUnit:       domain.ToolDurationUnitMonth,
	}

	t.Run("Exactly 2 months", func(t *testing.T) {
		start, _ := time.Parse("2006-01-02", "2024-01-15")
		end, _ := time.Parse("2006-01-02", "2024-03-14")
		breakdown, err := CalculateRentalCostWithBreakdown(start, end, tool)
		assert.NoError(t, err)
		assert.Equal(t, 2, breakdown.Months)
		assert.Equal(t, 0, breakdown.Weeks)
		assert.Equal(t, 0, breakdown.Days)
		assert.Equal(t, int32(27000), breakdown.TotalCost) // 2 * $135
	})

	t.Run("2 months + 6 days", func(t *testing.T) {
		start, _ := time.Parse("2006-01-02", "2024-01-15")
		end, _ := time.Parse("2006-01-02", "2024-03-20")
		breakdown, err := CalculateRentalCostWithBreakdown(start, end, tool)
		assert.NoError(t, err)
		assert.Equal(t, 3, breakdown.Months) // Rounded up
		assert.Equal(t, 0, breakdown.Weeks)
		assert.Equal(t, 0, breakdown.Days)
		assert.Equal(t, int32(40500), breakdown.TotalCost) // 3 * $135
	})
}

func TestCalculateRentalCostWithBreakdown_WeekUnit(t *testing.T) {
	tool := &domain.Tool{
		PricePerDayCents:   1000,  // $10.00
		PricePerWeekCents:  4500,  // $45.00
		PricePerMonthCents: 13500, // $135.00
		DurationUnit:       domain.ToolDurationUnitWeek,
	}

	t.Run("14 days = 2 weeks", func(t *testing.T) {
		start, _ := time.Parse("2006-01-02", "2024-01-15")
		end, _ := time.Parse("2006-01-02", "2024-01-28")
		breakdown, err := CalculateRentalCostWithBreakdown(start, end, tool)
		assert.NoError(t, err)
		assert.Equal(t, 0, breakdown.Months)
		assert.Equal(t, 2, breakdown.Weeks)
		assert.Equal(t, 0, breakdown.Days)
		assert.Equal(t, int32(9000), breakdown.TotalCost) // 2 * $45
	})

	t.Run("2 months + 10 days", func(t *testing.T) {
		start, _ := time.Parse("2006-01-02", "2024-01-15")
		end, _ := time.Parse("2006-01-02", "2024-03-24")
		breakdown, err := CalculateRentalCostWithBreakdown(start, end, tool)
		assert.NoError(t, err)
		assert.Equal(t, 2, breakdown.Months)
		assert.Equal(t, 2, breakdown.Weeks) // 10 days rounds up to 2 weeks
		assert.Equal(t, 0, breakdown.Days)
		assert.Equal(t, int32(36000), breakdown.TotalCost) // (2 * $135) + (2 * $45)
	})
}

func TestCalculateRentalCost_EdgeCases(t *testing.T) {
	tool := &domain.Tool{
		PricePerDayCents:   1000,
		PricePerWeekCents:  4500,
		PricePerMonthCents: 13500,
		DurationUnit:       domain.ToolDurationUnitDay,
	}

	t.Run("Leap year - February 29", func(t *testing.T) {
		start, _ := time.Parse("2006-01-02", "2024-02-15")
		end, _ := time.Parse("2006-01-02", "2024-03-14")
		cost, err := CalculateRentalCost(start, end, tool)
		assert.NoError(t, err)
		// Should calculate correctly across leap year February
		assert.NotZero(t, cost)
	})

	t.Run("Year boundary", func(t *testing.T) {
		start, _ := time.Parse("2006-01-02", "2023-12-25")
		end, _ := time.Parse("2006-01-02", "2024-01-10")
		cost, err := CalculateRentalCost(start, end, tool)
		assert.NoError(t, err)
		// 17 days = 2 weeks + 3 days
		assert.Equal(t, int32(12000), cost) // (2 * $45) + (3 * $10)
	})

	t.Run("Cross multiple years", func(t *testing.T) {
		start, _ := time.Parse("2006-01-02", "2023-01-15")
		end, _ := time.Parse("2006-01-02", "2025-01-14")
		cost, err := CalculateRentalCost(start, end, tool)
		assert.NoError(t, err)
		// Exactly 24 months
		assert.Equal(t, int32(324000), cost) // 24 * $135
	})
}

func TestCalculateRentalCost_DefaultUnit(t *testing.T) {
	t.Run("Empty duration unit defaults to day", func(t *testing.T) {
		tool := &domain.Tool{
			PricePerDayCents:   1000,
			PricePerWeekCents:  4500,
			PricePerMonthCents: 13500,
			DurationUnit:       "", // Empty/unset
		}

		start, _ := time.Parse("2006-01-02", "2024-01-15")
		end, _ := time.Parse("2006-01-02", "2024-01-21")
		cost, err := CalculateRentalCost(start, end, tool)
		assert.NoError(t, err)
		// 7 days = 1 week in day unit
		assert.Equal(t, int32(4500), cost) // 1 week * $45
	})
}
