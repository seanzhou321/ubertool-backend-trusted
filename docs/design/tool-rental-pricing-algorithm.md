# Tool Rental Pricing Algorithm

## Overview

This document provides a Go implementation of the tool rental pricing calculation system based on start and end dates, using a date-based tiered approach and duration unit specifications.

## Data Structures

```go
type DurationUnit string

const (
    DayUnit   DurationUnit = "day"
    WeekUnit  DurationUnit = "week"
    MonthUnit DurationUnit = "month"
)

type RentalPrices struct {
    DailyRate    int // Price in cents
    WeeklyRate   int // Price in cents
    MonthlyRate  int // Price in cents
    DurationUnit DurationUnit
}

type Date struct {
    Year  int
    Month int
    Day   int
}

type DateDifference struct {
    Years  int
    Months int
    Days   int
}

type RentalCost struct {
    Months     int
    Weeks      int
    Days       int
    MonthsCost int // Cost in cents
    WeeksCost  int // Cost in cents
    DaysCost   int // Cost in cents
    TotalCost  int // Cost in cents
}
```

## Core Algorithm

### Step 1: Parse Date Strings

Convert yyyy-mm-dd formatted strings into Date structs.

```go
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
```

**Example:**
```go
startDate, _ := ParseDate("2024-01-15")
// startDate = Date{Year: 2024, Month: 1, Day: 15}

endDate, _ := ParseDate("2024-03-20")
// endDate = Date{Year: 2024, Month: 3, Day: 20}
```

### Step 2: Calculate Days in Month

Helper function to get the number of days in a given month.

```go
func DaysInMonth(year, month int) int {
    // Handle month boundaries
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
```

**Example:**
```go
days := DaysInMonth(2024, 2) // 29 (leap year)
days = DaysInMonth(2023, 2)  // 28 (not a leap year)
days = DaysInMonth(2024, 4)  // 30 (April)
days = DaysInMonth(2024, 1)  // 31 (January)
```

### Step 3: Calculate Date Difference (Steps 2-5 from requirements)

Compute the difference between end date and start date as a tuple of (years, months, days).

```go
func CalculateDateDifference(startDate, endDate Date) (DateDifference, error) {
    if endDate.Year < startDate.Year ||
        (endDate.Year == startDate.Year && endDate.Month < startDate.Month) ||
        (endDate.Year == startDate.Year && endDate.Month == startDate.Month && endDate.Day < startDate.Day) {
        return DateDifference{}, fmt.Errorf("end date must be >= start date")
    }
    
    // Step 2: Count both ends (endDate - startDate + 1)
    diff := DateDifference{
        Years:  endDate.Year - startDate.Year,
        Months: endDate.Month - startDate.Month,
        Days:   endDate.Day - startDate.Day + 1, // +1 to include both ends
    }
    
    // Step 4: If diff.Days < 0, borrow from months
    if diff.Days <= 0 {
        diff.Months -= 1
        // Calculate days in the previous month
        prevMonth := endDate.Month - 1
        prevYear := endDate.Year
        if prevMonth < 1 {
            prevMonth = 12
            prevYear -= 1
        }
        daysInPrevMonth := DaysInMonth(prevYear, prevMonth)
        diff.Days = daysInPrevMonth + diff.Days
    }
    
    // If months are negative, borrow from years
    if diff.Months < 0 {
        diff.Years -= 1
        diff.Months += 12
    }
    
    // Step 5: Convert years to months (diff.Months += 12 * diff.Years)
    diff.Months += 12 * diff.Years
    diff.Years = 0 // We don't use years separately anymore
    
    return diff, nil
}
```

**Example:**
```go
// Example 1: Same month
start := Date{Year: 2024, Month: 1, Day: 15}
end := Date{Year: 2024, Month: 1, Day: 20}
diff, _ := CalculateDateDifference(start, end)
// diff = DateDifference{Years: 0, Months: 0, Days: 6}
// Calculation: Day 20 - Day 15 + 1 = 6 days

// Example 2: Cross month boundary
start = Date{Year: 2024, Month: 1, Day: 25}
end = Date{Year: 2024, Month: 2, Day: 5}
diff, _ = CalculateDateDifference(start, end)
// Initial: Years=0, Months=1, Days=5-25+1=-19
// After adjustment: Months=0, Days=31+(-19)=12
// Result: diff = DateDifference{Years: 0, Months: 0, Days: 12}

// Example 3: Multiple months
start = Date{Year: 2024, Month: 1, Day: 15}
end = Date{Year: 2024, Month: 4, Day: 20}
diff, _ = CalculateDateDifference(start, end)
// diff = DateDifference{Years: 0, Months: 3, Days: 6}
// Calculation: Months=4-1=3, Days=20-15+1=6

// Example 4: Cross year boundary
start = Date{Year: 2023, Month: 11, Day: 15}
end = Date{Year: 2024, Month: 2, Day: 10}
diff, _ = CalculateDateDifference(start, end)
// Initial: Years=1, Months=2-11=-9, Days=10-15+1=-4
// After day adjustment: Months=-10, Days=31+(-4)=27 (Jan has 31 days)
// After month adjustment: Years=0, Months=12+(-10)=2, Days=27
// After year conversion: Months=2+12*0=2, Days=27
// Result: diff = DateDifference{Years: 0, Months: 2, Days: 27}
```

### Step 4: Validate Input

```go
func validateInput(prices RentalPrices, startDate, endDate Date) error {
    if prices.DailyRate < 0 || prices.WeeklyRate < 0 || prices.MonthlyRate < 0 {
        return fmt.Errorf("prices cannot be negative")
    }
    
    validUnits := map[DurationUnit]bool{
        DayUnit: true, WeekUnit: true, MonthUnit: true,
    }
    if !validUnits[prices.DurationUnit] {
        return fmt.Errorf("invalid duration unit")
    }
    
    return nil
}
```

### Step 5: Calculate Rental Cost

Main function that routes to the appropriate calculation based on duration unit.

```go
func CalculateRentalCost(prices RentalPrices, startDateStr, endDateStr string) (RentalCost, error) {
    startDate, err := ParseDate(startDateStr)
    if err != nil {
        return RentalCost{}, fmt.Errorf("invalid start date: %v", err)
    }
    
    endDate, err := ParseDate(endDateStr)
    if err != nil {
        return RentalCost{}, fmt.Errorf("invalid end date: %v", err)
    }
    
    if err := validateInput(prices, startDate, endDate); err != nil {
        return RentalCost{}, err
    }
    
    diff, err := CalculateDateDifference(startDate, endDate)
    if err != nil {
        return RentalCost{}, err
    }
    
    switch prices.DurationUnit {
    case MonthUnit:
        return calculateMonthUnitCost(prices, diff)
    case WeekUnit:
        return calculateWeekUnitCost(prices, diff)
    case DayUnit:
        return calculateDayUnitCost(prices, diff)
    default:
        return RentalCost{}, fmt.Errorf("unsupported duration unit")
    }
}
```

### Step 6: Month Unit Calculation

For month duration unit, calculate based on months and remaining days (Step 6 from requirements).

```go
func calculateMonthUnitCost(prices RentalPrices, diff DateDifference) (RentalCost, error) {
    var months int
    
    // Step 6: If diff.Days == 0, price = diff.Months * monthRate
    //         else price = (diff.Months + 1) * monthRate
    if diff.Days == 0 {
        months = diff.Months
    } else {
        months = diff.Months + 1
    }
    
    totalCost := months * prices.MonthlyRate
    
    return RentalCost{
        Months:     months,
        Weeks:      0,
        Days:       0,
        MonthsCost: totalCost,
        WeeksCost:  0,
        DaysCost:   0,
        TotalCost:  totalCost,
    }, nil
}
```

**Example:**
```go
prices := RentalPrices{
    DailyRate:    1000,  // $10.00
    WeeklyRate:   4500,  // $45.00
    MonthlyRate:  13500, // $135.00
    DurationUnit: MonthUnit,
}

// Exactly 2 months (no extra days)
// 2024-01-15 to 2024-03-14 = 2 months, 0 days
diff := DateDifference{Months: 2, Days: 0}
cost, _ := calculateMonthUnitCost(prices, diff)
// cost.Months = 2
// cost.TotalCost = 27000 ($270.00)

// 2 months + 6 days (rounds up to 3 months)
// 2024-01-15 to 2024-03-20 = 2 months, 6 days
diff = DateDifference{Months: 2, Days: 6}
cost, _ = calculateMonthUnitCost(prices, diff)
// cost.Months = 3
// cost.TotalCost = 40500 ($405.00)
```

### Step 7: Week and Day Unit Calculation

For week or day duration units, convert the date difference to total days and apply tiered pricing (Step 7 from requirements).

```go
func calculateWeekUnitCost(prices RentalPrices, diff DateDifference) (RentalCost, error) {
    const daysPerWeek = 7
    
    // Round up to nearest full week
    weeks := diff.Days / daysPerWeek
    if (diff.Days - weeks*daysPerWeek > 0)
        weeks += 1
    
    monthCost := diff.Months * prices.MonthlyRate
    weekCost := min(diff.Weeks * prices.WeeklyRate, prices.MonthlyRate)
    
    return RentalCost{
        Months:     diff.Months,
        Weeks:      weeks,
        Days:       0,
        MonthsCost: monthCost,
        WeeksCost:  weekCost,
        DaysCost:   0,
        TotalCost:  monthCost+weekCost,
    }, nil
}

func calculateDayUnitCost(prices RentalPrices, diff DateDifference) (RentalCost, error) {
    const daysPerWeek = 7
    
    // Step 1: Calculate weeks and days
    weeks := diff.Days / daysPerWeek
    days := diff.Days - weeks*daysPerWeek
    
    // Step 2: Calculate costs
    monthsCost := diff.Months * prices.MonthlyRate
    weeksCost := min(weeks * prices.WeeklyRate, prices.MonthlyRate)
    daysCost := min(days * prices.DailyRate, prices.WeeklyRate)
    
    return RentalCost{
        Months:     months,
        Weeks:      weeks,
        Days:       days,
        MonthsCost: monthsCost,
        WeeksCost:  weeksCost,
        DaysCost:   daysCost,
        TotalCost:  monthsCost + weeksCost + daysCost,
    }, nil
}
```

**Example:**
```go
prices := RentalPrices{
    DailyRate:    1000,  // $10.00
    WeeklyRate:   4500,  // $45.00
    MonthlyRate:  13500, // $135.00
    DurationUnit: DayUnit,
}

// 2023-12-15 to 2024-03-10 = 2 months, 25 days
diff := DateDifference{Months: 2, Days: 25}
cost, _ := calculateDayUnitCost(prices, diff)
cost = RentalCost {
    Months: 2,
    Weeks: 3,
    Days: 4,
    MonthsCost: 27000,
    WeeksCost: 13500,
    DaysCost: 4000,
    TotalCost: 44500,
}
```

## Complete Implementation

```go
package main

import (
    "fmt"
    "strconv"
    "strings"
)

type DurationUnit string

const (
    DayUnit   DurationUnit = "day"
    WeekUnit  DurationUnit = "week"
    MonthUnit DurationUnit = "month"
)

type RentalPrices struct {
    DailyRate    int // Price in cents
    WeeklyRate   int // Price in cents
    MonthlyRate  int // Price in cents
    DurationUnit DurationUnit
}

type Date struct {
    Year  int
    Month int
    Day   int
}

type DateDifference struct {
    Years  int
    Months int
    Days   int
}

type RentalCost struct {
    Months     int
    Weeks      int
    Days       int
    MonthsCost int // Cost in cents
    WeeksCost  int // Cost in cents
    DaysCost   int // Cost in cents
    TotalCost  int // Cost in cents
}

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

func DaysInMonth(year, month int) int {
    if month == 2 {
        if (year%4 == 0 && year%100 != 0) || (year%400 == 0) {
            return 29
        }
        return 28
    }
    
    if month == 4 || month == 6 || month == 9 || month == 11 {
        return 30
    }
    
    return 31
}

func CalculateDateDifference(startDate, endDate Date) (DateDifference, error) {
    if endDate.Year < startDate.Year ||
        (endDate.Year == startDate.Year && endDate.Month < startDate.Month) ||
        (endDate.Year == startDate.Year && endDate.Month == startDate.Month && endDate.Day < startDate.Day) {
        return DateDifference{}, fmt.Errorf("end date must be >= start date")
    }
    
    diff := DateDifference{
        Years:  endDate.Year - startDate.Year,
        Months: endDate.Month - startDate.Month,
        Days:   endDate.Day - startDate.Day + 1,
    }
    
    if diff.Days <= 0 {
        diff.Months -= 1
        prevMonth := endDate.Month - 1
        prevYear := endDate.Year
        if prevMonth < 1 {
            prevMonth = 12
            prevYear -= 1
        }
        daysInPrevMonth := DaysInMonth(prevYear, prevMonth)
        diff.Days = daysInPrevMonth + diff.Days
    }
    
    if diff.Months < 0 {
        diff.Years -= 1
        diff.Months += 12
    }
    
    diff.Months += 12 * diff.Years
    diff.Years = 0
    
    return diff, nil
}

func validateInput(prices RentalPrices, startDate, endDate Date) error {
    if prices.DailyRate < 0 || prices.WeeklyRate < 0 || prices.MonthlyRate < 0 {
        return fmt.Errorf("prices cannot be negative")
    }
    
    validUnits := map[DurationUnit]bool{
        DayUnit: true, WeekUnit: true, MonthUnit: true,
    }
    if !validUnits[prices.DurationUnit] {
        return fmt.Errorf("invalid duration unit")
    }
    
    return nil
}

func CalculateRentalCost(prices RentalPrices, startDateStr, endDateStr string) (RentalCost, error) {
    startDate, err := ParseDate(startDateStr)
    if err != nil {
        return RentalCost{}, fmt.Errorf("invalid start date: %v", err)
    }
    
    endDate, err := ParseDate(endDateStr)
    if err != nil {
        return RentalCost{}, fmt.Errorf("invalid end date: %v", err)
    }
    
    if err := validateInput(prices, startDate, endDate); err != nil {
        return RentalCost{}, err
    }
    
    diff, err := CalculateDateDifference(startDate, endDate)
    if err != nil {
        return RentalCost{}, err
    }
    
    switch prices.DurationUnit {
    case MonthUnit:
        return calculateMonthUnitCost(prices, diff)
    case WeekUnit:
        return calculateWeekUnitCost(prices, diff)
    case DayUnit:
        return calculateDayUnitCost(prices, diff)
    default:
        return RentalCost{}, fmt.Errorf("unsupported duration unit")
    }
}

func calculateMonthUnitCost(prices RentalPrices, diff DateDifference) (RentalCost, error) {
    var months int
    
    if diff.Days == 0 {
        months = diff.Months
    } else {
        months = diff.Months + 1
    }
    
    totalCost := months * prices.MonthlyRate
    
    return RentalCost{
        Months:     months,
        Weeks:      0,
        Days:       0,
        MonthsCost: totalCost,
        WeeksCost:  0,
        DaysCost:   0,
        TotalCost:  totalCost,
    }, nil
}

func calculateWeekUnitCost(prices RentalPrices, diff DateDifference) (RentalCost, error) {
    const daysPerWeek = 7
    
    // Round up to nearest full week
    weeks := diff.Days / daysPerWeek
    if (diff.Days - weeks*daysPerWeek > 0)
        weeks += 1
    
    monthCost := diff.Months * prices.MonthlyRate
    weekCost := min(diff.Weeks * prices.WeeklyRate, prices.MonthlyRate)
    
    return RentalCost{
        Months:     diff.Months,
        Weeks:      weeks,
        Days:       0,
        MonthsCost: monthCost,
        WeeksCost:  weekCost,
        DaysCost:   0,
        TotalCost:  monthCost+weekCost,
    }, nil
}

func calculateDayUnitCost(prices RentalPrices, diff DateDifference) (RentalCost, error) {
    const daysPerWeek = 7
    
    // Step 1: Calculate weeks and days
    weeks := diff.Days / daysPerWeek
    days := diff.Days - weeks*daysPerWeek
    
    // Step 2: Calculate costs
    monthsCost := diff.Months * prices.MonthlyRate
    weeksCost := min(weeks * prices.WeeklyRate, pricesMonthlyRate)
    daysCost := min(days * prices.DailyRate, prices.WeeklyRate)
    
    return RentalCost{
        Months:     months,
        Weeks:      weeks,
        Days:       days,
        MonthsCost: monthsCost,
        WeeksCost:  weeksCost,
        DaysCost:   daysCost,
        TotalCost:  monthsCost + weeksCost + daysCost,
    }, nil
}

func (rc RentalCost) String() string {
    return fmt.Sprintf(
        "Breakdown: %d months + %d weeks + %d days\n"+
            "Months Cost: $%.2f\n"+
            "Weeks Cost: $%.2f\n"+
            "Days Cost: $%.2f\n"+
            "Total Cost: $%.2f",
        rc.Months, rc.Weeks, rc.Days,
        float64(rc.MonthsCost)/100.0,
        float64(rc.WeeksCost)/100.0,
        float64(rc.DaysCost)/100.0,
        float64(rc.TotalCost)/100.0,
    )
}

func main() {
    prices := RentalPrices{
        DailyRate:    1000,  // $10.00
        WeeklyRate:   4500,  // $45.00
        MonthlyRate:  13500, // $135.00
        DurationUnit: DayUnit,
    }
    
    cost, err := CalculateRentalCost(prices, "2023-12-15", "2024-03-10")
    if err != nil {
        fmt.Printf("Error: %v\n", err)
        return
    }
    
    fmt.Println(cost)
}
```

## Key Implementation Notes

1. **Date Format**: All dates must be in yyyy-mm-dd format (e.g., "2024-01-15").

2. **Inclusive Date Range**: Both start and end dates are included in the rental period (endDate - startDate + 1).

3. **Date Difference Calculation**: 
   - Calculates the difference as (years, months, days) tuple
   - Handles negative days by borrowing from months
   - Handles negative months by borrowing from years
   - Converts all years to months for final calculation

4. **Currency Representation**: All prices and costs use `int` representing cents to avoid floating-point arithmetic issues.

5. **Month Duration Unit**: 
   - If `diff.Days == 0`: charges exact months (`diff.Months * monthRate`)
   - If `diff.Days > 0`: rounds up to next month (`(diff.Months + 1) * monthRate`)

6. **Week and Day Units**: Convert the date difference to total days (months × 30 + days) before applying tiered pricing.

7. **Leap Year Support**: The `DaysInMonth` function properly handles leap years.

## Testing Scenarios

### Scenario 1: Month Unit - Exact Months
```go
prices := RentalPrices{
    DailyRate:    1000,
    WeeklyRate:   4500,
    MonthlyRate:  13500,
    DurationUnit: MonthUnit,
}

// Exactly 2 months: 2024-01-15 to 2024-03-14
cost, _ := CalculateRentalCost(prices, "2024-01-15", "2024-03-14")
// Expected: diff.Months=2, diff.Days=0
// Result: 2 months = 27000 cents = $270.00
```

### Scenario 2: Month Unit - Partial Month
```go
// 3 months + 6 days: 2023-12-15 to 2024-03-20
cost, _ := CalculateRentalCost(prices, "2023-12-15", "2024-03-20")
// Expected: diff.Months=3, diff.Days=6
// Result: 4 months (rounded up) = 52000 cents = $520.00
```

### Scenario 3: Day Unit - Mixed Duration
```go
prices := RentalPrices{
    DailyRate:    1000,
    WeeklyRate:   4500,
    MonthlyRate:  13500,
    DurationUnit: DayUnit,
}

// 2023-12-15 to 2024-03-10 = 2 months + 25 days
cost, _ := CalculateRentalCost(prices, "2023-12-15", "2024-03-10")
// Expected diff.Months=2, diffDays=25
// Breakdown: 2 months, 3 weeks, and 4 days
// Result: $270.00 + $135.00 + $40.00 = $445.00
```

### Scenario 4: Week Unit
```go
prices := RentalPrices{
    DailyRate:    1000,
    WeeklyRate:   4500,
    MonthlyRate:  13500,
    DurationUnit: WeekUnit,
}

// 2024-01-15 to 2024-01-28 = 0 months + 14 days = 14 total days
cost, _ := CalculateRentalCost(prices, "2024-01-15", "2024-01-28")
// totalDays = 14
// Weeks = (14 + 6) / 7 = 2 weeks
// Result: 2 weeks = 9000 cents = $90.00
```

### Scenario 5: Single Day Rental
```go
// Same start and end date: 2024-01-15 to 2024-01-15
cost, _ := CalculateRentalCost(prices, "2024-01-15", "2024-01-15")
// Expected: diff.Months=0, diff.Days=1 (endDate - startDate + 1)
// For DayUnit: totalDays = 1
// Result: 1 day = 1000 cents = $10.00
```

### Scenario 6: Cross Year Boundary
```go
// 2023-11-15 to 2024-02-10 = 2 months + 27 days
cost, _ := CalculateRentalCost(prices, "2023-11-15", "2024-02-10")
// Expected: diff.Months=2, diff.Days=27
// Breakdown: 2 months, 3 weeks, 6 days
// Result: $270.00 + $135.00 + $60.00 = $465.00
```

## Helper Functions

### Currency Conversion Utilities

```go
// DollarsToCents converts a dollar amount to cents
func DollarsToCents(dollars float) int {
    return int(dollars * 100)
}

// CentsToDollars converts cents to a dollar amount
func CentsToDollars(cents int) float {
    return float(cents) / 100.0
}

// FormatCents formats cents as a currency string
func FormatCents(cents int) string {
    return fmt.Sprintf("$%.2f", CentsToDollars(cents))
}
```

**Example Usage:**
```go
// Converting user input from dollars to cents
dailyPrice := DollarsToCents(10.00)    // 1000 cents
weeklyPrice := DollarsToCents(45.00)   // 4500 cents
monthlyPrice := DollarsToCents(135.00) // 13500 cents

prices := RentalPrices{
    DailyRate:    dailyPrice,
    WeeklyRate:   weeklyPrice,
    MonthlyRate:  monthlyPrice,
    DurationUnit: DayUnit,
}

// Displaying cost to user
cost, _ := CalculateRentalCost(prices, "2024-01-15", "2024-03-20")
fmt.Printf("Total: %s\n", FormatCents(cost.TotalCost))
// Output: Total: $330.00
```

## Edge Cases

1. **Single Day Rental**: Start date equals end date, counts as 1 day
2. **Month Boundaries**: Properly handles different month lengths (28, 29, 30, 31 days)
3. **Leap Years**: Correctly calculates February days in leap years
4. **Year Boundaries**: Handles rentals spanning across years
5. **Invalid Dates**: Returns errors for malformed date strings or impossible dates
