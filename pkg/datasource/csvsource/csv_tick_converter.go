package csvsource

import (
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

var klines []types.KLine

// Convert ticks to KLine with interval
func ConvertCsvTickToKLines(tick *CsvTick, interval KLineInterval) {
	var (
		currentCandle = types.KLine{}
		high          = fixedpoint.Zero
		low           = fixedpoint.Zero
		tickTimeStamp = time.Unix(tick.Timestamp, 0)
	)
	isOpen, t := detCandleStart(tickTimeStamp, interval)

	if isOpen {
		klines = append(klines, types.KLine{
			StartTime: types.NewTimeFromUnix(t.Unix(), 0),
			EndTime:   types.NewTimeFromUnix(t.Add(convertInterval(interval)).Unix(), 0),
			Open:      tick.Price,
			High:      tick.Price,
			Low:       tick.Price,
			Close:     tick.Price,
			Volume:    tick.HomeNotional,
		})
		return
	}

	currentCandle = klines[len(klines)-1]

	if tick.Price.Float64() > currentCandle.High.Float64() {
		high = tick.Price
	} else {
		high = currentCandle.High
	}

	if tick.Price.Float64() < currentCandle.Low.Float64() {
		low = tick.Price
	} else {
		low = currentCandle.Low
	}

	klines[len(klines)-1] = types.KLine{
		StartTime: currentCandle.StartTime,
		EndTime:   currentCandle.EndTime,
		Open:      currentCandle.Open,
		High:      high,
		Low:       low,
		Close:     tick.Price,
		Volume:    currentCandle.Volume.Add(tick.HomeNotional),
	}
}

func detCandleStart(ts time.Time, kInterval KLineInterval) (isOpen bool, t time.Time) {
	if len(klines) == 0 {
		return true, convertTimestamp(ts, kInterval)
	}
	var (
		current = klines[len(klines)-1]
		end     = current.EndTime.Time()
	)
	if ts.After(end) {
		return true, end
	}

	return false, t
}
