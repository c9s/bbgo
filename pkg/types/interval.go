package types

import "encoding/json"

type Interval string

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

var Interval1m = Interval("1m")
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

var SupportedIntervals = map[Interval]int{
	Interval1m:  1,
	Interval5m:  5,
	Interval15m: 15,
	Interval30m: 30,
	Interval1h:  60,
	Interval2h:  60 * 2,
	Interval4h:  60 * 4,
	Interval6h:  60 * 6,
	Interval12h: 60 * 12,
	Interval1d:  60 * 24,
	Interval3d:  60 * 24 * 3,
}
