package calendar_test

import (
	"testing"
	"time"

	"github.com/sonroyaalmerol/snry-shell/internal/calendar"
)

func TestBuildMonthGridApril2026(t *testing.T) {
	grid := calendar.BuildMonthGrid(2026, time.April)

	if len(grid) != 6 {
		t.Fatalf("expected 6 rows, got %d", len(grid))
	}
	for i, row := range grid {
		if len(row) != 7 {
			t.Fatalf("row %d: expected 7 cols, got %d", i, len(row))
		}
	}

	// April 1 2026 is a Wednesday → offset = 3, so row[0][0] = Sun Mar 29
	first := grid[0][0]
	if first.Weekday() != time.Sunday {
		t.Fatalf("first cell must be Sunday, got %s", first.Weekday())
	}

	// April 1 should be in row 0, col 3 (Wed)
	april1 := grid[0][3]
	if april1.Month() != time.April || april1.Day() != 1 {
		t.Fatalf("expected April 1 at [0][3], got %s", april1)
	}

	// Last cell of the grid should be in May
	last := grid[5][6]
	if last.Month() != time.May {
		t.Fatalf("last cell should be in May, got %s", last.Month())
	}
}

func TestBuildMonthGridJanuary2026(t *testing.T) {
	grid := calendar.BuildMonthGrid(2026, time.January)

	// Jan 1 2026 is a Thursday → offset = 4, so row[0][0] = Sun Dec 28 2025
	first := grid[0][0]
	if first.Weekday() != time.Sunday {
		t.Fatalf("first cell must be Sunday, got %s", first.Weekday())
	}
	if first.Month() != time.December || first.Year() != 2025 {
		t.Fatalf("expected Dec 2025, got %s", first)
	}
}

func TestBuildMonthGridAlwaysSixRows(t *testing.T) {
	months := []struct {
		year  int
		month time.Month
	}{
		{2026, time.February},
		{2026, time.March},
		{2026, time.June},
		{2025, time.February},
	}
	for _, m := range months {
		grid := calendar.BuildMonthGrid(m.year, m.month)
		if len(grid) != 6 {
			t.Errorf("%d/%d: expected 6 rows, got %d", m.year, m.month, len(grid))
		}
	}
}

func TestIsCurrentMonth(t *testing.T) {
	d := time.Date(2026, time.April, 15, 0, 0, 0, 0, time.Local)
	if !calendar.IsCurrentMonth(d, 2026, time.April) {
		t.Fatal("expected true for same month")
	}
	if calendar.IsCurrentMonth(d, 2026, time.March) {
		t.Fatal("expected false for different month")
	}
}

func TestDayHeaders(t *testing.T) {
	headers := calendar.DayHeaders()
	if len(headers) != 7 {
		t.Fatalf("expected 7 headers, got %d", len(headers))
	}
	if headers[0] != "Su" {
		t.Fatalf("first header must be Su, got %q", headers[0])
	}
	if headers[6] != "Sa" {
		t.Fatalf("last header must be Sa, got %q", headers[6])
	}
}
