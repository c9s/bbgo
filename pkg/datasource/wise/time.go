package wise

import (
	"encoding/json"
	"time"
)

const layout = "2006-01-02T15:04:05-0700"

type Time time.Time

func (t Time) Time() time.Time {
	return time.Time(t)
}

func (t Time) String() string {
	return time.Time(t).Format(layout)
}

func (t *Time) UnmarshalJSON(data []byte) error {
	var s string
	err := json.Unmarshal(data, &s)
	if err != nil {
		return err
	}

	parsed, err := time.Parse(layout, s)
	if err != nil {
		return err
	}

	*t = Time(parsed)
	return nil
}
