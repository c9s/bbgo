package csvsource

import (
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type ICSVTickConverter interface {
	CsvTickToKLine(tick *CsvTick) (closesKLine bool)
	GetTicks() []*CsvTick
	LatestKLine(interval types.Interval) (k *types.KLine)
	GetKLineResults() map[types.Interval][]types.KLine
}

// CSVTickConverter takes a tick and internally converts it to a KLine slice
type CSVTickConverter struct {
	ticks     []*CsvTick
	intervals []types.Interval
	klines    map[types.Interval][]types.KLine
}

func NewCSVTickConverter(intervals []types.Interval) ICSVTickConverter {
	return &CSVTickConverter{
		ticks:     []*CsvTick{},
		intervals: intervals,
		klines:    make(map[types.Interval][]types.KLine),
	}
}

func (c *CSVTickConverter) GetTicks() []*CsvTick {
	return c.ticks
}

func (c *CSVTickConverter) AddKLine(interval types.Interval, k types.KLine) {
	c.klines[interval] = append(c.klines[interval], k)
}

// GetKLineResult returns the converted ticks as kLine of interval
func (c *CSVTickConverter) LatestKLine(interval types.Interval) (k *types.KLine) {
	if _, ok := c.klines[interval]; !ok || len(c.klines[interval]) == 0 {
		return nil
	}
	return &c.klines[interval][len(c.klines[interval])-1]
}

// GetKLineResults returns the converted ticks as kLine of all constructed intervals
func (c *CSVTickConverter) GetKLineResults() map[types.Interval][]types.KLine {
	if len(c.klines) == 0 {
		return nil
	}
	return c.klines
}

// Convert ticks to KLine with interval
func (c *CSVTickConverter) CsvTickToKLine(tick *CsvTick) (closesKLine bool) {
	for _, interval := range c.intervals {
		var (
			currentCandle = types.KLine{}
			high          = fixedpoint.Zero
			low           = fixedpoint.Zero
		)
		isOpen, t := c.detCandleStart(tick.Timestamp.Time(), interval)
		if isOpen {
			latestKline := c.LatestKLine(interval)
			if latestKline != nil {
				latestKline.Closed = true // k is pointer
				closesKLine = true
				c.addMissingKLines(interval, t)
			}
			c.AddKLine(interval, types.KLine{
				Exchange:    tick.Exchange,
				Symbol:      tick.Symbol,
				Interval:    interval,
				StartTime:   types.NewTimeFromUnix(t.Unix(), 0),
				EndTime:     types.NewTimeFromUnix(t.Add(interval.Duration()).Unix(), 0),
				Open:        tick.Price,
				High:        tick.Price,
				Low:         tick.Price,
				Close:       tick.Price,
				Volume:      tick.HomeNotional,
				QuoteVolume: tick.ForeignNotional,
				Closed:      false,
			})

			return
		}

		currentCandle = c.klines[interval][len(c.klines[interval])-1]

		if tick.Price.Compare(currentCandle.High) > 0 {
			high = tick.Price
		} else {
			high = currentCandle.High
		}

		if tick.Price.Compare(currentCandle.Low) < 0 {
			low = tick.Price
		} else {
			low = currentCandle.Low
		}

		c.klines[interval][len(c.klines[interval])-1] = types.KLine{
			StartTime:   currentCandle.StartTime,
			EndTime:     currentCandle.EndTime,
			Exchange:    tick.Exchange,
			Symbol:      tick.Symbol,
			Interval:    interval,
			Open:        currentCandle.Open,
			High:        high,
			Low:         low,
			Close:       tick.Price,
			Volume:      currentCandle.Volume.Add(tick.HomeNotional),
			QuoteVolume: currentCandle.QuoteVolume.Add(tick.ForeignNotional),
			Closed:      false,
		}
	}

	return
}

func (c *CSVTickConverter) detCandleStart(ts time.Time, interval types.Interval) (isOpen bool, t time.Time) {
	if len(c.klines) == 0 {
		return true, interval.Truncate(ts)
	}

	var end = c.LatestKLine(interval).EndTime.Time()
	if ts.After(end) {
		return true, end
	}

	return false, t
}

// appendMissingKLines appends an empty kline till startNext falls within a kline interval
func (c *CSVTickConverter) addMissingKLines(
	interval types.Interval,
	startNext time.Time,
) {
	for {
		last := c.LatestKLine(interval)
		newEndTime := types.NewTimeFromUnix(
			// one second is the smallest interval
			last.EndTime.Time().Add(time.Duration(last.Interval.Seconds())*time.Second).Unix(),
			0,
		)
		if last.EndTime.Time().Before(startNext) {
			c.AddKLine(interval, types.KLine{
				StartTime:   last.EndTime,
				EndTime:     newEndTime,
				Exchange:    last.Exchange,
				Symbol:      last.Symbol,
				Interval:    last.Interval,
				Open:        last.Close,
				High:        last.Close,
				Low:         last.Close,
				Close:       last.Close,
				Volume:      0,
				QuoteVolume: 0,
				Closed:      true,
			})
		} else {
			break
		}
	}
}
