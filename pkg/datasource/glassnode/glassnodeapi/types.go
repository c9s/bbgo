package glassnodeapi

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

/*
In Glassnode API, there are two types of response, for example:

	/v1/metrics/market/marketcap_usd

	[
		{
			"t": 1614556800,
			"v": 927789865185.0476
		},
		...
	]

and

	/v1/metrics/market/price_usd_ohlc

	[
		{
			"t": 1614556800,
			"o": {
				"c": 49768.16035012147,
				"h": 49773.18922304233,
				"l": 45159.50305252744,
				"o": 45159.50305252744
			}
		},
		...
	]

both can be stored into the DataSlice structure.

Note: use `HasOptions` to verify the type of response.
*/
type DataSlice []Data
type Data struct {
	Timestamp Timestamp          `json:"t"`
	Value     float64            `json:"v"`
	Options   map[string]float64 `json:"o"`
}

func (s DataSlice) IsEmpty() bool {
	return len(s) == 0
}

func (s DataSlice) First() Data {
	if s.IsEmpty() {
		return Data{}
	}
	return s[0]
}
func (s DataSlice) FirstValue() float64 {
	return s.First().Value
}

func (s DataSlice) FirstOptions() map[string]float64 {
	return s.First().Options
}

func (s DataSlice) Last() Data {
	if s.IsEmpty() {
		return Data{}
	}
	return s[len(s)-1]
}

func (s DataSlice) LastValue() float64 {
	return s.Last().Value
}

func (s DataSlice) LastOptions() map[string]float64 {
	return s.Last().Options
}

func (s DataSlice) HasOptions() bool {
	return len(s.First().Options) != 0
}
