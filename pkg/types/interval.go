package types

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

type Interval string

func (i Interval) Minutes() int {
	m, ok := SupportedIntervals[i]
	if !ok {
		return ParseInterval(i) / 60
	}
	return m / 60
}

func (i Interval) Seconds() int {
	m, ok := SupportedIntervals[i]
	if !ok {
		return ParseInterval(i)
	}
	return m
}

// Milliseconds is specially handled, for better precision
// for ms level interval, calling Seconds and Minutes directly might trigger panic error
func (i Interval) Milliseconds() int {
	t := 0
	index := 0
	for i, rn := range string(i) {
		if rn >= '0' && rn <= '9' {
			t = t*10 + int(rn-'0')
		} else {
			index = i
			break
		}
	}
	switch strings.ToLower(string(i[index:])) {
	case "ms":
		return t
	case "s":
		return t * 1000
	case "m":
		t *= 60
	case "h":
		t *= 60 * 60
	case "d":
		t *= 60 * 60 * 24
	case "w":
		t *= 60 * 60 * 24 * 7
	case "mo":
		t *= 60 * 60 * 24 * 30
	default:
		panic("unknown interval input: " + i)
	}
	return t * 1000
}

func (i Interval) Duration() time.Duration {
	return time.Duration(i.Milliseconds()) * time.Millisecond
}

// Truncate determines the candle open time from a given timestamp
// eg interval 1 hour and tick at timestamp 00:58:45 will return timestamp shifted to 00:00:00
func (i Interval) Truncate(ts time.Time) (start time.Time) {
	switch i {
	case Interval1s:
		return ts.Truncate(time.Second)
	case Interval1m:
		return ts.Truncate(time.Minute)
	case Interval3m:
		return shiftMinute(ts, 3)
	case Interval5m:
		return shiftMinute(ts, 5)
	case Interval15m:
		return shiftMinute(ts, 15)
	case Interval30m:
		return shiftMinute(ts, 30)
	case Interval1h:
		return ts.Truncate(time.Hour)
	case Interval2h:
		return shiftHour(ts, 2)
	case Interval4h:
		return shiftHour(ts, 4)
	case Interval6h:
		return shiftHour(ts, 6)
	case Interval12h:
		return shiftHour(ts, 12)
	case Interval1d:
		return ts.Truncate(time.Hour * 24)
	case Interval3d:
		return shiftDay(ts, 3)
	case Interval1w:
		return shiftDay(ts, 7)
	case Interval2w:
		return shiftDay(ts, 14)
	case Interval1mo:
		return time.Date(ts.Year(), ts.Month(), 0, 0, 0, 0, 0, time.UTC)
	}
	return start
}

func shiftDay(ts time.Time, shift int) time.Time {
	day := ts.Day() - (ts.Day() % shift)
	return time.Date(ts.Year(), ts.Month(), day, 0, 0, 0, 0, ts.Location())
}

func shiftHour(ts time.Time, shift int) time.Time {
	hour := ts.Hour() - (ts.Hour() % shift)
	return time.Date(ts.Year(), ts.Month(), ts.Day(), hour, 0, 0, 0, ts.Location())
}

func shiftMinute(ts time.Time, shift int) time.Time {
	minute := ts.Minute() - (ts.Minute() % shift)
	return time.Date(ts.Year(), ts.Month(), ts.Day(), ts.Hour(), minute, 0, 0, ts.Location())
}

func (i *Interval) UnmarshalJSON(b []byte) (err error) {
	var a string
	err = json.Unmarshal(b, &a)
	if err != nil {
		return err
	}

	*i = Interval(a)
	return
}

func (i Interval) String() string {
	return string(i)
}

type IntervalSlice []Interval

func (s IntervalSlice) Sort() {
	sort.Slice(s, func(i, j int) bool {
		return s[i].Duration() < s[j].Duration()
	})
}

func (s IntervalSlice) StringSlice() (slice []string) {
	for _, interval := range s {
		slice = append(slice, `"`+interval.String()+`"`)
	}
	return slice
}

var Interval1s = Interval("1s")
var Interval1m = Interval("1m")
var Interval3m = Interval("3m")
var Interval5m = Interval("5m")
var Interval15m = Interval("15m")
var Interval30m = Interval("30m")
var Interval1h = Interval("1h")
var Interval2h = Interval("2h")
var Interval4h = Interval("4h")
var Interval6h = Interval("6h")
var Interval12h = Interval("12h")
var Interval1d = Interval("1d")
var Interval3d = Interval("3d")
var Interval1w = Interval("1w")
var Interval2w = Interval("2w")
var Interval1mo = Interval("1mo")

func ParseInterval(input Interval) int {
	t := 0
	index := 0
	for i, rn := range string(input) {
		if rn >= '0' && rn <= '9' {
			t = t*10 + int(rn-'0')
		} else {
			index = i
			break
		}
	}
	switch strings.ToLower(string(input[index:])) {
	case "s":
		return t
	case "m":
		t *= 60
	case "h":
		t *= 60 * 60
	case "d":
		t *= 60 * 60 * 24
	case "w":
		t *= 60 * 60 * 24 * 7
	case "mo":
		t *= 60 * 60 * 24 * 30
	default:
		panic("unknown interval input: " + input)
	}
	return t
}

type IntervalMap map[Interval]int

func (m IntervalMap) Slice() (slice IntervalSlice) {
	for interval := range m {
		slice = append(slice, interval)
	}

	return slice
}

var SupportedIntervals = IntervalMap{
	Interval1s:  1,
	Interval1m:  1 * 60,
	Interval3m:  3 * 60,
	Interval5m:  5 * 60,
	Interval15m: 15 * 60,
	Interval30m: 30 * 60,
	Interval1h:  60 * 60,
	Interval2h:  60 * 60 * 2,
	Interval4h:  60 * 60 * 4,
	Interval6h:  60 * 60 * 6,
	Interval12h: 60 * 60 * 12,
	Interval1d:  60 * 60 * 24,
	Interval3d:  60 * 60 * 24 * 3,
	Interval1w:  60 * 60 * 24 * 7,
	Interval2w:  60 * 60 * 24 * 14,
	Interval1mo: 60 * 60 * 24 * 30,
}

// IntervalWindow is used by the indicators
type IntervalWindow struct {
	// The interval of kline
	Interval Interval `json:"interval"`

	// The windows size of the indicator (for example, EWMA and SMA)
	Window int `json:"window"`

	// RightWindow is used by the pivot indicator
	RightWindow *int `json:"rightWindow"`
}

type IntervalWindowBandWidth struct {
	IntervalWindow
	BandWidth float64 `json:"bandWidth"`
}

func (iw IntervalWindow) String() string {
	return fmt.Sprintf("%s (%d)", iw.Interval, iw.Window)
}
