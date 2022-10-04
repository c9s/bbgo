package types

import (
	"encoding/json"
	"fmt"
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

func (i Interval) Duration() time.Duration {
	return time.Duration(i.Seconds()) * time.Second
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
		t *= 3600
	case "d":
		t *= 3600 * 24
	case "w":
		t *= 3600 * 24 * 7
	case "mo":
		t *= 3600 * 24 * 30
	default:
		panic("unknown input: " + input)
	}
	return t
}

var SupportedIntervals = map[Interval]int{
	Interval1s:  1,
	Interval1m:  60,
	Interval3m:  180,
	Interval5m:  300,
	Interval15m: 900,
	Interval30m: 1800,
	Interval1h:  3600,
	Interval2h:  3600 * 2,
	Interval4h:  3600 * 4,
	Interval6h:  3600 * 6,
	Interval12h: 3600 * 12,
	Interval1d:  3600 * 24,
	Interval3d:  3600 * 24 * 3,
	Interval1w:  3600 * 24 * 7,
	Interval2w:  3600 * 24 * 14,
	Interval1mo: 3600 * 24 * 30,
}

// IntervalWindow is used by the indicators
type IntervalWindow struct {
	// The interval of kline
	Interval Interval `json:"interval"`

	// The windows size of the indicator (for example, EWMA and SMA)
	Window int `json:"window"`

	// RightWindow is used by the pivot indicator
	RightWindow int `json:"rightWindow"`
}

type IntervalWindowBandWidth struct {
	IntervalWindow
	BandWidth float64 `json:"bandWidth"`
}

func (iw IntervalWindow) String() string {
	return fmt.Sprintf("%s (%d)", iw.Interval, iw.Window)
}
