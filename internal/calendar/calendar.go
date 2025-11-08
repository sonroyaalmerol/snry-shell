package calendar

import "time"

// BuildMonthGrid returns a 6-row × 7-col grid of dates for the given year and month.
// The week starts on Monday (ISO 8601). Days from the previous or next month
// are included to fill the grid.
func BuildMonthGrid(year int, month time.Month) [][]time.Time {
	first := time.Date(year, month, 1, 0, 0, 0, 0, time.Local)
	// Weekday offset: Monday = 0 … Sunday = 6
	offset := int(first.Weekday()+6) % 7
	start := first.AddDate(0, 0, -offset)

	grid := make([][]time.Time, 6)
	day := start
	for row := range 6 {
		week := make([]time.Time, 7)
		for col := range 7 {
			week[col] = day
			day = day.AddDate(0, 0, 1)
		}
		grid[row] = week
	}
	return grid
}

// IsToday reports whether t falls on the current calendar date.
func IsToday(t time.Time) bool {
	now := time.Now()
	return t.Year() == now.Year() && t.Month() == now.Month() && t.Day() == now.Day()
}

// IsCurrentMonth reports whether t is in the given year/month.
func IsCurrentMonth(t time.Time, year int, month time.Month) bool {
	return t.Year() == year && t.Month() == month
}

// DayHeaders returns the abbreviated day names starting from Monday.
func DayHeaders() []string {
	return []string{"Mo", "Tu", "We", "Th", "Fr", "Sa", "Su"}
}
