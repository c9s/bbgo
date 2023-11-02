package csvsource

import (
	"encoding/csv"
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type ICSVTickConverter interface {
	LatestKLine() (k *types.KLine)
	GetKLineResult() []types.KLine
	CsvTickToKLine(tick *CsvTick, interval types.Interval)
}

// CSVTickConverter takes a tick and internally converts it to a KLine slice
type CSVTickConverter struct {
	csv     *csv.Reader
	decoder CSVTickDecoder
	klines  []types.KLine
}

func NewCSVTickConverter() ICSVTickConverter {
	return &CSVTickConverter{
		klines: []types.KLine{},
	}
}

// GetKLineResult returns the converted ticks as kLine of interval
func (c *CSVTickConverter) LatestKLine() (k *types.KLine) {
	if len(c.klines) == 0 {
		return nil
	}
	return &c.klines[len(c.klines)-1]
}

// GetKLineResult returns the converted ticks as kLine of interval
func (c *CSVTickConverter) GetKLineResult() []types.KLine {
	return c.klines
}

// Convert ticks to KLine with interval
func (c *CSVTickConverter) CsvTickToKLine(tick *CsvTick, interval types.Interval) {
	var (
		currentCandle = types.KLine{}
		high          = fixedpoint.Zero
		low           = fixedpoint.Zero
	)
	isOpen, t := c.detCandleStart(tick.Timestamp.Time(), interval)

	if isOpen {
		c.klines = append(c.klines, types.KLine{
			StartTime: types.NewTimeFromUnix(t.Unix(), 0),
			EndTime:   types.NewTimeFromUnix(t.Add(interval.Duration()).Unix(), 0),
			Open:      tick.Price,
			High:      tick.Price,
			Low:       tick.Price,
			Close:     tick.Price,
			Volume:    tick.HomeNotional,
		})
		return
	}

	currentCandle = c.klines[len(c.klines)-1]

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

	c.klines[len(c.klines)-1] = types.KLine{
		StartTime: currentCandle.StartTime,
		EndTime:   currentCandle.EndTime,
		Open:      currentCandle.Open,
		High:      high,
		Low:       low,
		Close:     tick.Price,
		Volume:    currentCandle.Volume.Add(tick.HomeNotional),
	}
}

func (c *CSVTickConverter) detCandleStart(ts time.Time, interval types.Interval) (isOpen bool, t time.Time) {
	if len(c.klines) == 0 {
		return true, interval.Convert(ts)
	}
	var (
		current = c.klines[len(c.klines)-1]
		end     = current.EndTime.Time()
	)
	if ts.After(end) {
		return true, end
	}

	return false, t
}
