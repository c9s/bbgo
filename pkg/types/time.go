package types

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

type MillisecondTimestamp time.Time

func NewMillisecondTimestampFromInt(i int64) MillisecondTimestamp {
	return MillisecondTimestamp(time.Unix(0, i * int64(time.Millisecond)))
}

func MustParseMillisecondTimestamp(a string) MillisecondTimestamp {
	m, err := strconv.ParseInt(a, 10, 64) // startTime
	if err != nil {
		panic(fmt.Errorf("millisecond timestamp parse error %v", err))
	}

	return NewMillisecondTimestampFromInt(m)
}

func MustParseUnixTimestamp(a string) time.Time {
	m, err := strconv.ParseInt(a, 10, 64) // startTime
	if err != nil {
		panic(fmt.Errorf("millisecond timestamp parse error %v", err))
	}

	return time.Unix(m, 0)
}

func (t MillisecondTimestamp) String() string {
	return time.Time(t).String()
}

func (t MillisecondTimestamp) Time() time.Time {
	return time.Time(t)
}

func (t *MillisecondTimestamp) UnmarshalJSON(data []byte) error {
	var v interface{}

	var err = json.Unmarshal(data, &v)
	if err != nil {
		return err
	}

	switch vt := v.(type) {
	case string:
		if vt == "" {
			// treat empty string as 0
			*t = MillisecondTimestamp(time.Time{})
			return nil
		}

		i, err := strconv.ParseInt(vt, 10, 64)
		if err == nil {
			*t = MillisecondTimestamp(time.Unix(0, i*int64(time.Millisecond)))
			return nil
		}

		f, err := strconv.ParseFloat(vt, 64)
		if err == nil {
			*t = MillisecondTimestamp(time.Unix(0, int64(f*float64(time.Millisecond))))
			return nil
		}

		tt, err := time.Parse(time.RFC3339Nano, vt)
		if err == nil {
			*t = MillisecondTimestamp(tt)
			return nil
		}

		return err

	case int64:
		*t = MillisecondTimestamp(time.Unix(0, vt*int64(time.Millisecond)))
		return nil

	case int:
		*t = MillisecondTimestamp(time.Unix(0, int64(vt)*int64(time.Millisecond)))
		return nil

	case float64:
		*t = MillisecondTimestamp(time.Unix(0, int64(vt)*int64(time.Millisecond)))
		return nil

	default:
		return fmt.Errorf("can not parse %T %+v as millisecond timestamp", vt, vt)

	}

	// fallback to RFC3339
	return (*time.Time)(t).UnmarshalJSON(data)
}

type Time time.Time

var layout = "2006-01-02 15:04:05.999Z07:00"

func (t *Time) UnmarshalJSON(data []byte) error {
	// fallback to RFC3339
	return (*time.Time)(t).UnmarshalJSON(data)
}

func (t Time) MarshalJSON() ([]byte, error) {
	return time.Time(t).MarshalJSON()
}

func (t Time) String() string {
	return time.Time(t).String()
}

func (t Time) Time() time.Time {
	return time.Time(t)
}

func (t Time) Unix() int64 {
	return time.Time(t).Unix()
}

func (t Time) After(time2 time.Time) bool {
	return time.Time(t).After(time2)
}

func (t Time) Before(time2 time.Time) bool {
	return time.Time(t).Before(time2)
}

func NewTimeFromUnix(sec int64, nsec int64) Time {
	return Time(time.Unix(sec, nsec))
}

// Value implements the driver.Valuer interface
// see http://jmoiron.net/blog/built-in-interfaces/
func (t Time) Value() (driver.Value, error) {
	return time.Time(t), nil
}

func (t *Time) Scan(src interface{}) error {
	switch d := src.(type) {

	case *time.Time:
		*t = Time(*d)
		return nil

	case time.Time:
		*t = Time(d)
		return nil

	case string:
		// 2020-12-16 05:17:12.994+08:00
		tt, err := time.Parse(layout, d)
		if err != nil {
			return err
		}

		*t = Time(tt)
		return nil

	case []byte:
		// 2019-10-20 23:01:43.77+08:00
		tt, err := time.Parse(layout, string(d))
		if err != nil {
			return err
		}

		*t = Time(tt)
		return nil

	default:

	}

	return fmt.Errorf("datatype.Time scan error, type: %T is not supported, value; %+v", src, src)
}
