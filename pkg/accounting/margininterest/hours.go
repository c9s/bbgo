// Package margininterest estimates margin interest in hours.
//
// Binance charges simple interest on principal at the top of every UTC hour,
// for the hour that starts then. The unit of every estimate is therefore one
// hourly charge, and a month is only a way to choose a number of hours:
//
// An HourRange [Start, End) on UTC hour boundaries is the base window; the
// month helpers only build one:
//
//   - the calendar month: MonthRange(now), split at now (SplitAt). Its
//     interest is the realized charges in [MonthStart, floor(now)+1h) plus the
//     projection over the hours after floor(now) until the month end.
//   - the rolling month: 30 days, RollingMonthHours. "The next n months" is a
//     projection over RollingMonthsHours(n) hours from now, with no realized part.
package margininterest

import "time"

// HoursPerDay is the number of hourly charges in a day.
const HoursPerDay = 24

// RollingMonthHours is the number of hourly charges in a rolling month of 30 days.
const RollingMonthHours = 30 * HoursPerDay

// RollingMonthsHours returns the number of hourly charges in n rolling months.
func RollingMonthsHours(n int) int {
	if n <= 0 {
		return 0
	}
	return n * RollingMonthHours
}

// HourRange is the hours [Start, End), both at the top of a UTC hour. It holds
// the hourly charges at Start, Start+1h, ..., End-1h, and any prorated
// borrow-time charge stamped inside those hours.
type HourRange struct {
	Start time.Time
	End   time.Time
}

// NewHourRange truncates start and end down to the top of their UTC hour.
func NewHourRange(start, end time.Time) HourRange {
	return HourRange{
		Start: start.UTC().Truncate(time.Hour),
		End:   end.UTC().Truncate(time.Hour),
	}
}

// Hours returns the number of hourly charges in the range, 0 if End is not after Start.
func (r HourRange) Hours() int {
	if !r.End.After(r.Start) {
		return 0
	}
	return int(r.End.Sub(r.Start) / time.Hour)
}

// SplitAt splits r at now into the realized hours, [Start, floor(now)+1h), and
// the number of hourly charges after floor(now), up to End. The charge at
// floor(now) has already happened, so it is realized. Every hour of r is in
// exactly one part.
func (r HourRange) SplitAt(now time.Time) (realized HourRange, remaining int) {
	split := now.UTC().Truncate(time.Hour).Add(time.Hour)
	if split.Before(r.Start) {
		split = r.Start
	}
	if split.After(r.End) {
		split = r.End
	}

	return HourRange{Start: r.Start, End: split}, HourRange{Start: split, End: r.End}.Hours()
}

// MonthRange returns the hours of t's UTC calendar month.
func MonthRange(t time.Time) HourRange {
	return HourRange{Start: MonthStart(t), End: MonthEnd(t)}
}

// MonthStart returns 00:00:00 UTC on day 1 of t's UTC month.
func MonthStart(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
}

// MonthEnd returns the start of the UTC month after t's UTC month.
func MonthEnd(t time.Time) time.Time {
	return MonthStart(t).AddDate(0, 1, 0)
}

// HoursInMonth returns the number of hourly charges in t's UTC month.
func HoursInMonth(t time.Time) int {
	return MonthRange(t).Hours()
}

// RemainingHoursInMonth returns the number of top-of-hour charges strictly
// after now, up to the end of now's UTC calendar month, see SplitAt.
func RemainingHoursInMonth(now time.Time) int {
	_, remaining := MonthRange(now).SplitAt(now)
	return remaining
}
