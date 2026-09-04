package backpackapi

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

// MicrosecondTimestamp decodes a unix timestamp in microseconds.
//
// Backpack reports the matching engine timestamp of the order book in microseconds, which none
// of the shared bbgo timestamp types cover.
type MicrosecondTimestamp time.Time

func NewMicrosecondTimestampFromInt(us int64) MicrosecondTimestamp {
	return MicrosecondTimestamp(time.Unix(0, us*int64(time.Microsecond)))
}

func (t MicrosecondTimestamp) Time() time.Time {
	return time.Time(t)
}

func (t MicrosecondTimestamp) String() string {
	return time.Time(t).String()
}

func (t *MicrosecondTimestamp) UnmarshalJSON(data []byte) error {
	var v interface{}
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}

	switch vt := v.(type) {
	case nil:
		*t = MicrosecondTimestamp(time.Time{})
		return nil

	case float64:
		*t = NewMicrosecondTimestampFromInt(int64(vt))
		return nil

	case string:
		if len(vt) == 0 {
			*t = MicrosecondTimestamp(time.Time{})
			return nil
		}

		us, err := strconv.ParseInt(vt, 10, 64)
		if err != nil {
			return err
		}

		*t = NewMicrosecondTimestampFromInt(us)
		return nil
	}

	return fmt.Errorf("backpackapi: unsupported microsecond timestamp type %T: %v", v, v)
}

// NullableMillisecondTimestamp decodes a unix millisecond timestamp that the API may send as
// null.
//
// types.MillisecondTimestamp rejects a null literal, and the order endpoints send null for the
// timestamps of the conditional order fields that do not apply.
type NullableMillisecondTimestamp time.Time

func (t NullableMillisecondTimestamp) Time() time.Time {
	return time.Time(t)
}

func (t NullableMillisecondTimestamp) String() string {
	return time.Time(t).String()
}

func (t NullableMillisecondTimestamp) IsZero() bool {
	return time.Time(t).IsZero()
}

func (t *NullableMillisecondTimestamp) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*t = NullableMillisecondTimestamp(time.Time{})
		return nil
	}

	var ts types.MillisecondTimestamp
	if err := ts.UnmarshalJSON(data); err != nil {
		return err
	}

	*t = NullableMillisecondTimestamp(ts.Time())
	return nil
}

// klineTimeLayout is the layout of the kline start/end fields: a space separated UTC datetime
// without a timezone suffix, e.g. "2026-09-01 17:23:00".
const klineTimeLayout = "2006-01-02 15:04:05"

// KlineTime decodes the kline start/end timestamps.
//
// These are not covered by types.Time: its loose formats only accept the "T" separated form.
type KlineTime time.Time

func (t KlineTime) Time() time.Time {
	return time.Time(t)
}

func (t KlineTime) String() string {
	return time.Time(t).String()
}

func (t *KlineTime) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	if len(s) == 0 {
		*t = KlineTime(time.Time{})
		return nil
	}

	// the API is documented to return UTC without a zone suffix, but tolerate a full RFC3339
	// value as well so that a future format change does not break decoding.
	tv, err := time.Parse(klineTimeLayout, s)
	if err != nil {
		tv, err = time.Parse(time.RFC3339, s)
		if err != nil {
			return fmt.Errorf("backpackapi: unable to parse kline time %q: %w", s, err)
		}
	}

	*t = KlineTime(tv)
	return nil
}

// PriceLevel is a single order book level, encoded by the API as a ["price", "quantity"] pair.
type PriceLevel struct {
	Price    fixedpoint.Value
	Quantity fixedpoint.Value
}

func (l *PriceLevel) UnmarshalJSON(data []byte) error {
	var pair []fixedpoint.Value
	if err := json.Unmarshal(data, &pair); err != nil {
		return err
	}

	if len(pair) != 2 {
		return fmt.Errorf("backpackapi: expected a [price, quantity] pair, got %d elements: %s",
			len(pair), string(data))
	}

	l.Price = pair[0]
	l.Quantity = pair[1]
	return nil
}

func (l PriceLevel) String() string {
	return fmt.Sprintf("%s@%s", l.Quantity.String(), l.Price.String())
}
