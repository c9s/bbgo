package types

import (
	"database/sql/driver"
	"fmt"
	"time"
)

type Time time.Time

var layout = "2006-01-02 15:04:05.999Z07:00"

func (t *Time) UnmarshalJSON(data []byte) error {
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

// driver.Valuer interface
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
