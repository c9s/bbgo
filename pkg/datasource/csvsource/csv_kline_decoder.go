package csvsource

import (
	"strconv"
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

// parseCsvKLineRecord parse a CSV record into a KLine.
func parseCsvKLineRecord(record []string, interval time.Duration) (types.KLine, error) {
	var (
		k, empty types.KLine
		err      error
	)
	if len(record) < 5 {
		return k, ErrNotEnoughColumns
	}
	ts, err := strconv.ParseFloat(record[0], 64) // check for e numbers "1.70027E+12"
	if err != nil {
		return empty, ErrInvalidTimeFormat
	}
	open, err := fixedpoint.NewFromString(record[1])
	if err != nil {
		return empty, ErrInvalidPriceFormat
	}
	high, err := fixedpoint.NewFromString(record[2])
	if err != nil {
		return empty, ErrInvalidPriceFormat
	}
	low, err := fixedpoint.NewFromString(record[3])
	if err != nil {
		return empty, ErrInvalidPriceFormat
	}
	closing, err := fixedpoint.NewFromString(record[4])
	if err != nil {
		return empty, ErrInvalidPriceFormat
	}

	volume := fixedpoint.Zero
	if len(record) >= 6 {
		volume, err = fixedpoint.NewFromString(record[5])
		if err != nil {
			return empty, ErrInvalidVolumeFormat
		}
	}

	// ts is in milliseconds, convert to seconds and nanoseconds
	tsMs := int64(ts)
	tsSec := tsMs / 1000
	tsNsec := (tsMs % 1000) * 1000000

	k.StartTime = types.NewTimeFromUnix(tsSec, tsNsec)
	k.EndTime = types.NewTimeFromUnix(k.StartTime.Time().Add(interval).Unix(), 0)
	k.Open = open
	k.High = high
	k.Low = low
	k.Close = closing
	k.Volume = volume

	return k, nil
}
