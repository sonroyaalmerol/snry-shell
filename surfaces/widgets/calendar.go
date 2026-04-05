package widgets

import (
	"fmt"
	"time"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/calendar"
	"github.com/sonroyaalmerol/snry-shell/internal/gtkutil"
	"github.com/sonroyaalmerol/snry-shell/internal/surfaceutil"
)

func BuildCalendarGroup() gtk.Widgetter {
	box := gtk.NewBox(gtk.OrientationVertical, 0)
	box.AddCSSClass("calendar-group")

	now := time.Now()
	year, month, _ := now.Date()
	yearVal, monthVal := year, month

	// Header: month/year + chevron buttons.
	header := gtk.NewBox(gtk.OrientationHorizontal, 0)
	header.SetHAlign(gtk.AlignFill)

	prevBtn := gtkutil.M3IconButton("chevron_left", "cal-nav-btn")

	monthLabel := gtk.NewLabel("")
	monthLabel.AddCSSClass("cal-month-label")
	monthLabel.SetHExpand(true)
	monthLabel.SetHAlign(gtk.AlignCenter)

	nextBtn := gtkutil.M3IconButton("chevron_right", "cal-nav-btn")

	header.Append(prevBtn)
	header.Append(monthLabel)
	header.Append(nextBtn)
	box.Append(header)

	// Day-of-week headers.
	dowRow := gtk.NewBox(gtk.OrientationHorizontal, 0)
	dowRow.AddCSSClass("cal-dow-row")
	for _, h := range calendar.DayHeaders() {
		l := gtk.NewLabel(h)
		l.AddCSSClass("cal-dow-label")
		l.SetHExpand(true)
		l.SetHAlign(gtk.AlignCenter)
		dowRow.Append(l)
	}
	box.Append(dowRow)

	// Grid for dates.
	grid := gtk.NewGrid()
	grid.AddCSSClass("cal-grid")
	grid.SetRowSpacing(2)
	grid.SetColumnSpacing(2)
	box.Append(grid)

	buildGrid := func() {
		for c := grid.FirstChild(); c != nil; {
			widget := surfaceutil.AsWidget(c)
				var next gtk.Widgetter
				if widget != nil {
					next = widget.NextSibling()
				}
			grid.Remove(c)
			c = next
		}
		monthLabel.SetText(fmt.Sprintf("%s %d", monthVal.String(), yearVal))
		days := calendar.BuildMonthGrid(yearVal, monthVal)
		for row, week := range days {
			for col, day := range week {
				l := gtk.NewLabel(fmt.Sprintf("%d", day.Day()))
				l.AddCSSClass("cal-day")
				l.SetHAlign(gtk.AlignCenter)
				l.SetHExpand(true)
				if calendar.IsToday(day) {
					l.AddCSSClass("today")
				}
				if !calendar.IsCurrentMonth(day, yearVal, monthVal) {
					l.AddCSSClass("other-month")
				}
				grid.Attach(l, col, row, 1, 1)
			}
		}
	}

	prevBtn.ConnectClicked(func() {
		if monthVal == time.January {
			monthVal = time.December
			yearVal--
		} else {
			monthVal--
		}
		buildGrid()
	})

	nextBtn.ConnectClicked(func() {
		if monthVal == time.December {
			monthVal = time.January
			yearVal++
		} else {
			monthVal++
		}
		buildGrid()
	})

	buildGrid()
	return box
}
