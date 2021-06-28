package indicator

import (
	"math"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/types"
)

const MaxNumOfEWMA = 1_000
const MaxNumOfEWMATruncateSize = 500

//go:generate callbackgen -type EWMA
type EWMA struct {
	types.IntervalWindow
	Values       types.Float64Slice
	LastOpenTime time.Time

	UpdateCallbacks []func(value float64)
}

func (inc *EWMA) Update(value float64) {
	var multiplier = 2.0 / float64(1+inc.Window)

	if len(inc.Values) == 0 {
		inc.Values.Push(value)
		return
	} else if len(inc.Values) > MaxNumOfEWMA {
		inc.Values = inc.Values[MaxNumOfEWMATruncateSize:]
	}

	ema := (1-multiplier)*inc.Last() + multiplier*value
	inc.Values.Push(ema)
}

func (inc *EWMA) Last() float64 {
	if len(inc.Values) == 0 {
		return 0
	}

	return inc.Values[len(inc.Values)-1]
}

func (inc *EWMA) calculateAndUpdate(allKLines []types.KLine) {
	if len(allKLines) < inc.Window {
		// we can't calculate
		return
	}

	var priceF = KLineClosePriceMapper
	var dataLen = len(allKLines)
	var multiplier = 2.0 / (float64(inc.Window) + 1)

	// init the values from the kline data
	var from = 1
	if len(inc.Values) == 0 {
		// for the first value, we should use the close price
		inc.Values = []float64{priceF(allKLines[0])}
	} else {
		// from = len(inc.Values)

		// update ewma with the existing values
		for i := dataLen - 1; i > 0; i-- {
			var k = allKLines[i]
			if k.StartTime.After(inc.LastOpenTime) {
				from = i
			} else {
				break
			}
		}
	}

	for i := from; i < dataLen; i++ {
		var k = allKLines[i]
		var ewma = priceF(k)*multiplier + (1-multiplier)*inc.Values[i-1]
		inc.Values.Push(ewma)
		inc.LastOpenTime = k.StartTime
		inc.EmitUpdate(ewma)
	}

	if len(inc.Values) != dataLen {
		log.Warnf("%s EMA (%d) value length (%d) != all kline data length (%d)", inc.Interval, inc.Window, len(inc.Values), dataLen)
	}

	v1 := math.Floor(inc.Values[len(inc.Values)-1]*100.0) / 100.0
	v2 := math.Floor(CalculateKLinesEMA(allKLines, priceF, inc.Window)*100.0) / 100.0
	if v1 != v2 {
		log.Warnf("ACCUMULATED %s EMA (%d) %f != EMA %f", inc.Interval, inc.Window, v1, v2)
	}
}

func CalculateKLinesEMA(allKLines []types.KLine, priceF KLinePriceMapper, window int) float64 {
	var multiplier = 2.0 / (float64(window) + 1)
	return ewma(MapKLinePrice(allKLines, priceF), multiplier)
}

// see https://www.investopedia.com/ask/answers/122314/what-exponential-moving-average-ema-formula-and-how-ema-calculated.asp
func ewma(prices []float64, multiplier float64) float64 {
	var end = len(prices) - 1
	if end == 0 {
		return prices[0]
	}

	return prices[end]*multiplier + (1-multiplier)*ewma(prices[:end], multiplier)
}

type KLinePriceMapper func(k types.KLine) float64

func KLineOpenPriceMapper(k types.KLine) float64 {
	return k.Open
}

func KLineClosePriceMapper(k types.KLine) float64 {
	return k.Close
}

func KLineTypicalPriceMapper(k types.KLine) float64 {
	return (k.High + k.Low + k.Close) / float64(3)
}

func MapKLinePrice(kLines []types.KLine, f KLinePriceMapper) (prices []float64) {
	for _, k := range kLines {
		prices = append(prices, f(k))
	}

	return prices
}

type KLineWindowUpdater interface {
	OnKLineWindowUpdate(func(interval types.Interval, window types.KLineWindow))
}

func (inc *EWMA) handleKLineWindowUpdate(interval types.Interval, window types.KLineWindow) {
	if inc.Interval != interval {
		return
	}

	inc.calculateAndUpdate(window)
}

func (inc *EWMA) Bind(updater KLineWindowUpdater) {
	updater.OnKLineWindowUpdate(inc.handleKLineWindowUpdate)
}
