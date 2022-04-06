package glassnode

import (
	"encoding/json"
	"time"
)

type Interval string

const (
	Interval1h  Interval = "1h"
	Interval24h Interval = "24h"
	Interval10m Interval = "10m"
	Interval1w  Interval = "1w"
	Interval1m  Interval = "1month"
)

type Format string

const (
	FormatJSON Format = "JSON"
	FormatCSV  Format = "CSV"
)

type Timestamp time.Time

func (t Timestamp) Unix() float64 {
	return float64(time.Time(t).Unix())
}

func (t Timestamp) String() string {
	return time.Time(t).String()
}

func (t *Timestamp) UnmarshalJSON(o []byte) error {
	var timestamp int64
	if err := json.Unmarshal(o, &timestamp); err != nil {
		return err
	}

	*t = Timestamp(time.Unix(timestamp, 0))
	return nil
}

// [{"t":1614556800,"v":927789865185.0476}]
type Response []Data
type Data struct {
	Timestamp Timestamp          `json:"t"`
	Value     float64            `json:"v"`
	Options   map[string]float64 `json:"o"`
}

func (s Response) Last() Data {
	if len(s) == 0 {
		return Data{}
	}
	return s[len(s)-1]
}

func (s Response) LastValue() float64 {
	return s.Last().Value
}

func (s Response) LastOptions() map[string]float64 {
	return s.Last().Options
}
