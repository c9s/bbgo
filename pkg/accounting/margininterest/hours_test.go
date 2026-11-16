package margininterest

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func mustTime(t *testing.T, s string) time.Time {
	t.Helper()
	tt, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatal(err)
	}
	return tt
}

func TestHoursInMonth(t *testing.T) {
	tests := map[string]int{
		"2026-11-10T00:00:00Z": 720,
		"2026-12-10T00:00:00Z": 744,
		"2026-02-10T00:00:00Z": 672,
		"2028-02-10T00:00:00Z": 696,
	}
	for in, want := range tests {
		assert.Equal(t, want, HoursInMonth(mustTime(t, in)), in)
	}
}

func TestRemainingHoursInMonth(t *testing.T) {
	tests := []struct {
		now  string
		want int
	}{
		{"2026-11-15T10:20:00Z", 373},
		{"2026-11-30T23:30:00Z", 0},
		{"2026-11-01T00:00:00Z", 719},
		{"2026-11-30T22:59:59Z", 1},
		// 2026-11-15T04:50Z
		{"2026-11-15T10:20:00+05:30", 379},
		// 2026-11-30T21:30Z: still November in UTC
		{"2026-12-01T03:00:00+05:30", 2},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, RemainingHoursInMonth(mustTime(t, tt.now)), tt.now)
	}
}

func TestMonthStartEnd(t *testing.T) {
	now := mustTime(t, "2026-12-01T03:00:00+05:30")
	assert.Equal(t, time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), MonthStart(now))
	assert.Equal(t, time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC), MonthEnd(now))

	// the 00:00 charge on day 1 belongs to the new month
	now = mustTime(t, "2026-12-01T00:00:00Z")
	assert.Equal(t, time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC), MonthStart(now))
}

func TestRollingMonthsHours(t *testing.T) {
	assert.Equal(t, 720, RollingMonthHours)
	assert.Equal(t, 720, RollingMonthsHours(1))
	assert.Equal(t, 2160, RollingMonthsHours(3))
	assert.Equal(t, 0, RollingMonthsHours(0))
	assert.Equal(t, 0, RollingMonthsHours(-1))
}

func TestHourRange(t *testing.T) {
	r := NewHourRange(mustTime(t, "2026-11-15T10:20:00+05:30"), mustTime(t, "2026-11-15T12:59:59Z"))
	assert.Equal(t, time.Date(2026, 11, 15, 4, 0, 0, 0, time.UTC), r.Start)
	assert.Equal(t, time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC), r.End)
	assert.Equal(t, 8, r.Hours())

	assert.Equal(t, 0, NewHourRange(r.End, r.Start).Hours())
	assert.Equal(t, 0, NewHourRange(r.Start, r.Start).Hours())
}

func TestMonthRange(t *testing.T) {
	// 2026-11-30T21:30Z: still November in UTC
	r := MonthRange(mustTime(t, "2026-12-01T03:00:00+05:30"))
	assert.Equal(t, time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), r.Start)
	assert.Equal(t, time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC), r.End)
	assert.Equal(t, 720, r.Hours())
}

func TestHourRange_SplitAt(t *testing.T) {
	month := MonthRange(mustTime(t, "2026-11-15T10:20:00Z"))
	h := func(s string) time.Time { return mustTime(t, s) }

	tests := []struct {
		now       string
		end       string
		remaining int
	}{
		// the charge at floor(now) is realized
		{"2026-11-15T10:20:00Z", "2026-11-15T11:00:00Z", 373},
		{"2026-11-15T10:00:00Z", "2026-11-15T11:00:00Z", 373},
		{"2026-11-01T00:00:00Z", "2026-11-01T01:00:00Z", 719},
		{"2026-11-30T23:30:00Z", "2026-12-01T00:00:00Z", 0},
		// now outside the range
		{"2026-10-20T00:00:00Z", "2026-11-01T00:00:00Z", 720},
		{"2026-12-05T00:00:00Z", "2026-12-01T00:00:00Z", 0},
	}
	for _, tt := range tests {
		realized, remaining := month.SplitAt(h(tt.now))
		assert.Equal(t, month.Start, realized.Start, tt.now)
		assert.Equal(t, h(tt.end), realized.End, tt.now)
		assert.Equal(t, tt.remaining, remaining, tt.now)
		// every hour of the month is in exactly one part
		assert.Equal(t, month.Hours(), realized.Hours()+remaining, tt.now)
	}
}
