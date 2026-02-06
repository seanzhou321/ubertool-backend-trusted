package utils

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"ubertool-backend-trusted/internal/domain"
)

// Date represents a calendar date
type Date struct {
	Year  int
	Month int
	Day   int
}

// DateDifference represents the difference between two dates
type DateDifference struct {
	Months int
	Days   int
}

// RentalCostBreakdown provides detailed cost breakdown
type RentalCostBreakdown struct {
	Months     int
	Weeks      int
	Days       int
	MonthsCost int32
	WeeksCost  int32
	DaysCost   int32
	TotalCost  int32
}

// ParseDate converts a yyyy-mm-dd formatted string into a Date struct
func ParseDate(dateStr string) (Date, error) {
	parts := strings.Split(dateStr, "-")
	if len(parts) != 3 {
		return Date{}, fmt.Errorf("invalid date format, expected yyyy-mm-dd")
	}

	year, err := strconv.Atoi(parts[0])
	if err != nil {
		return Date{}, fmt.Errorf("invalid year: %v", err)
	}

	month, err := strconv.Atoi(parts[1])
	if err != nil {
		return Date{}, fmt.Errorf("invalid month: %v", err)
	}

	day, err := strconv.Atoi(parts[2])
	if err != nil {
		return Date{}, fmt.Errorf("invalid day: %v", err)
	}

	if month < 1 || month > 12 {
		return Date{}, fmt.Errorf("month must be between 1 and 12")
	}

	if day < 1 || day > 31 {
		return Date{}, fmt.Errorf("day must be between 1 and 31")
	}

	return Date{Year: year, Month: month, Day: day}, nil
}

// DaysInMonth returns the number of days in a given month
func DaysInMonth(year, month int) int {
	if month == 2 {
		// Check for leap year
		if (year%4 == 0 && year%100 != 0) || (year%400 == 0) {
			return 29
		}
		return 28
	}

	// Months with 30 days: April, June, September, November
	if month == 4 || month == 6 || month == 9 || month == 11 {
		return 30
	}

	// All other months have 31 days
	return 31
}

// CalculateDateDifference computes the difference between two dates
// Returns (months, days) where both start and end dates are included
func CalculateDateDifference(startDate, endDate Date) (DateDifference, error) {
	if endDate.Year < startDate.Year ||
		(endDate.Year == startDate.Year && endDate.Month < startDate.Month) ||
		(endDate.Year == startDate.Year && endDate.Month == startDate.Month && endDate.Day < startDate.Day) {
		return DateDifference{}, fmt.Errorf("end date must be >= start date")
	}

	// Initial difference calculation
	years := endDate.Year - startDate.Year
	months := endDate.Month - startDate.Month
	days := endDate.Day - startDate.Day + 1 // +1 to include both ends

	// If days < 0, borrow from months
	if days < 0 {
		months -= 1
		// Calculate days in the previous month
		prevMonth := endDate.Month - 1
		prevYear := endDate.Year
		if prevMonth < 1 {
			prevMonth = 12
			prevYear -= 1
		}
		daysInPrevMonth := DaysInMonth(prevYear, prevMonth)
		days = daysInPrevMonth + days
	}

	// If months are negative, borrow from years
	if months < 0 {
		years -= 1
		months += 12
	}

	// Convert years to months
	months += 12 * years

	return DateDifference{Months: months, Days: days}, nil
}

// CalculateRentalCost calculates the total rental cost based on the tool's pricing
// and duration unit, following the tiered pricing algorithm from tool-rental-pricing-algorithm.md
func CalculateRentalCost(startDate, endDate time.Time, tool *domain.Tool) (int32, error) {
	// Convert time.Time to yyyy-mm-dd format strings
	startDateStr := startDate.Format("2006-01-02")
	endDateStr := endDate.Format("2006-01-02")

	// Parse dates
	start, err := ParseDate(startDateStr)
	if err != nil {
		return 0, fmt.Errorf("invalid start date: %v", err)
	}

	end, err := ParseDate(endDateStr)
	if err != nil {
		return 0, fmt.Errorf("invalid end date: %v", err)
	}

	// Calculate date difference
	diff, err := CalculateDateDifference(start, end)
	if err != nil {
		return 0, err
	}

	// Calculate cost based on duration unit
	switch tool.DurationUnit {
	case domain.ToolDurationUnitMonth:
		return calculateMonthUnitCost(tool, diff), nil
	case domain.ToolDurationUnitWeek:
		return calculateWeekUnitCost(tool, diff), nil
	case domain.ToolDurationUnitDay:
		return calculateDayUnitCost(tool, diff), nil
	default:
		// Default to day unit if not specified
		return calculateDayUnitCost(tool, diff), nil
	}
}

// calculateMonthUnitCost calculates cost for month-based duration
// If diff.Days == 0, charges exact months; otherwise rounds up to next month
func calculateMonthUnitCost(tool *domain.Tool, diff DateDifference) int32 {
	var months int32
	if diff.Days == 0 {
		months = int32(diff.Months)
	} else {
		months = int32(diff.Months + 1)
	}

	if months < 1 {
		months = 1
	}

	return months * tool.PricePerMonthCents
}

// calculateWeekUnitCost calculates cost for week-based duration
// Rounds up to nearest full week
func calculateWeekUnitCost(tool *domain.Tool, diff DateDifference) int32 {
	const daysPerWeek = 7

	// Calculate total weeks, rounding up
	weeks := int32(diff.Days / daysPerWeek)
	if diff.Days%daysPerWeek > 0 {
		weeks += 1
	}

	// Add cost for full months (converted to weeks)
	monthsCost := int32(diff.Months) * tool.PricePerMonthCents
	weeksCost := weeks * tool.PricePerWeekCents

	return monthsCost + weeksCost
}

// calculateDayUnitCost calculates cost using tiered pricing (months + weeks + days)
func calculateDayUnitCost(tool *domain.Tool, diff DateDifference) int32 {
	const daysPerWeek = 7

	// Break down remaining days into weeks and days
	weeks := int32(diff.Days / daysPerWeek)
	days := int32(diff.Days % daysPerWeek)

	// Calculate costs
	monthsCost := int32(diff.Months) * tool.PricePerMonthCents
	weeksCost := weeks * tool.PricePerWeekCents
	daysCost := days * tool.PricePerDayCents

	return monthsCost + weeksCost + daysCost
}

// CalculateRentalCostWithBreakdown provides detailed breakdown of rental cost
func CalculateRentalCostWithBreakdown(startDate, endDate time.Time, tool *domain.Tool) (RentalCostBreakdown, error) {
	// Convert time.Time to yyyy-mm-dd format strings
	startDateStr := startDate.Format("2006-01-02")
	endDateStr := endDate.Format("2006-01-02")

	// Parse dates
	start, err := ParseDate(startDateStr)
	if err != nil {
		return RentalCostBreakdown{}, fmt.Errorf("invalid start date: %v", err)
	}

	end, err := ParseDate(endDateStr)
	if err != nil {
		return RentalCostBreakdown{}, fmt.Errorf("invalid end date: %v", err)
	}

	// Calculate date difference
	diff, err := CalculateDateDifference(start, end)
	if err != nil {
		return RentalCostBreakdown{}, err
	}

	// Calculate breakdown based on duration unit
	switch tool.DurationUnit {
	case domain.ToolDurationUnitMonth:
		var months int
		if diff.Days == 0 {
			months = diff.Months
		} else {
			months = diff.Months + 1
		}
		if months < 1 {
			months = 1
		}
		totalCost := int32(months) * tool.PricePerMonthCents
		return RentalCostBreakdown{
			Months:     months,
			Weeks:      0,
			Days:       0,
			MonthsCost: totalCost,
			WeeksCost:  0,
			DaysCost:   0,
			TotalCost:  totalCost,
		}, nil

	case domain.ToolDurationUnitWeek:
		const daysPerWeek = 7
		weeks := diff.Days / daysPerWeek
		if diff.Days%daysPerWeek > 0 {
			weeks += 1
		}
		monthsCost := int32(diff.Months) * tool.PricePerMonthCents
		weeksCost := int32(weeks) * tool.PricePerWeekCents
		return RentalCostBreakdown{
			Months:     diff.Months,
			Weeks:      weeks,
			Days:       0,
			MonthsCost: monthsCost,
			WeeksCost:  weeksCost,
			DaysCost:   0,
			TotalCost:  monthsCost + weeksCost,
		}, nil

	case domain.ToolDurationUnitDay:
		const daysPerWeek = 7
		weeks := diff.Days / daysPerWeek
		days := diff.Days % daysPerWeek
		monthsCost := int32(diff.Months) * tool.PricePerMonthCents
		weeksCost := int32(weeks) * tool.PricePerWeekCents
		daysCost := int32(days) * tool.PricePerDayCents
		return RentalCostBreakdown{
			Months:     diff.Months,
			Weeks:      weeks,
			Days:       days,
			MonthsCost: monthsCost,
			WeeksCost:  weeksCost,
			DaysCost:   daysCost,
			TotalCost:  monthsCost + weeksCost + daysCost,
		}, nil

	default:
		// Default to day unit
		const daysPerWeek = 7
		weeks := diff.Days / daysPerWeek
		days := diff.Days % daysPerWeek
		monthsCost := int32(diff.Months) * tool.PricePerMonthCents
		weeksCost := int32(weeks) * tool.PricePerWeekCents
		daysCost := int32(days) * tool.PricePerDayCents
		return RentalCostBreakdown{
			Months:     diff.Months,
			Weeks:      weeks,
			Days:       days,
			MonthsCost: monthsCost,
			WeeksCost:  weeksCost,
			DaysCost:   daysCost,
			TotalCost:  monthsCost + weeksCost + daysCost,
		}, nil
	}
}
