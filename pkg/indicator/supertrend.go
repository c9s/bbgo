package indicator

import (
	"math"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/types"
)

var logst = logrus.WithField("indicator", "supertrend")

// The Super Trend is a technical analysis indicator that is used to identify potential buy and sell signals in a security's price. It is
// calculated by combining the exponential moving average (EMA) and the average true range (ATR) of the security's price, and then plotting
// the resulting value on the price chart as a line. The Super Trend line is typically used to identify potential entry and exit points
// for trades, and can be used to confirm other technical analysis signals. It is typically more responsive to changes in the underlying
// data than other trend-following indicators, but may be less reliable in trending markets. It is important to note that the Super Trend is a
// lagging indicator, which means that it may not always provide accurate or timely signals.
//
// To use Super Trend, identify potential entry and exit points for trades by looking for crossovers or divergences between the Super Trend line
// and the security's price. For example, a buy signal may be generated when the Super Trend line crosses above the security's price, while a sell
// signal may be generated when the Super Trend line crosses below the security's price.

//go:generate callbackgen -type Supertrend
type Supertrend struct {
	types.SeriesBase
	types.IntervalWindow
	ATRMultiplier float64 `json:"atrMultiplier"`

	AverageTrueRange *ATR

	trendPrices    floats.Slice // Value of the trend line (buy or sell)
	supportLine    floats.Slice // The support line in an uptrend (green)
	resistanceLine floats.Slice // The resistance line in a downtrend (red)

	closePrice             float64
	previousClosePrice     float64
	uptrendPrice           float64
	previousUptrendPrice   float64
	downtrendPrice         float64
	previousDowntrendPrice float64

	trend         types.Direction
	previousTrend types.Direction
	tradeSignal   types.Direction

	EndTime         time.Time
	UpdateCallbacks []func(value float64)
}

func (inc *Supertrend) Last(i int) float64 {
	return inc.trendPrices.Last(i)
}

func (inc *Supertrend) Index(i int) float64 {
	return inc.Last(i)
}

func (inc *Supertrend) Length() int {
	return len(inc.trendPrices)
}

func (inc *Supertrend) Update(highPrice, lowPrice, closePrice float64) {
	if inc.Window <= 0 {
		panic("window must be greater than 0")
	}

	if inc.AverageTrueRange == nil {
		inc.SeriesBase.Series = inc
	}

	// Start with DirectionUp
	if inc.trend != types.DirectionUp && inc.trend != types.DirectionDown {
		inc.trend = types.DirectionUp
	}

	// Update ATR
	inc.AverageTrueRange.Update(highPrice, lowPrice, closePrice)

	// Update last prices
	inc.previousUptrendPrice = inc.uptrendPrice
	inc.previousDowntrendPrice = inc.downtrendPrice
	inc.previousClosePrice = inc.closePrice
	inc.previousTrend = inc.trend

	inc.closePrice = closePrice

	src := (highPrice + lowPrice) / 2

	// Update uptrend
	inc.uptrendPrice = src - inc.AverageTrueRange.Last(0)*inc.ATRMultiplier
	if inc.previousClosePrice > inc.previousUptrendPrice {
		inc.uptrendPrice = math.Max(inc.uptrendPrice, inc.previousUptrendPrice)
	}

	// Update downtrend
	inc.downtrendPrice = src + inc.AverageTrueRange.Last(0)*inc.ATRMultiplier
	if inc.previousClosePrice < inc.previousDowntrendPrice {
		inc.downtrendPrice = math.Min(inc.downtrendPrice, inc.previousDowntrendPrice)
	}

	// Update trend
	if inc.previousTrend == types.DirectionUp && inc.closePrice < inc.previousUptrendPrice {
		inc.trend = types.DirectionDown
	} else if inc.previousTrend == types.DirectionDown && inc.closePrice > inc.previousDowntrendPrice {
		inc.trend = types.DirectionUp
	} else {
		inc.trend = inc.previousTrend
	}

	// Update signal
	if inc.AverageTrueRange.Last(0) <= 0 {
		inc.tradeSignal = types.DirectionNone
	} else if inc.trend == types.DirectionUp && inc.previousTrend == types.DirectionDown {
		inc.tradeSignal = types.DirectionUp
	} else if inc.trend == types.DirectionDown && inc.previousTrend == types.DirectionUp {
		inc.tradeSignal = types.DirectionDown
	} else {
		inc.tradeSignal = types.DirectionNone
	}

	// Update trend price
	if inc.trend == types.DirectionDown {
		inc.trendPrices.Push(inc.downtrendPrice)
	} else {
		inc.trendPrices.Push(inc.uptrendPrice)
	}

	// Save the trend lines
	inc.supportLine.Push(inc.uptrendPrice)
	inc.resistanceLine.Push(inc.downtrendPrice)

	logst.Debugf("Update supertrend result: closePrice: %v, uptrendPrice: %v, downtrendPrice: %v, trend: %v,"+
		" tradeSignal: %v, AverageTrueRange.Last(): %v", inc.closePrice, inc.uptrendPrice, inc.downtrendPrice,
		inc.trend, inc.tradeSignal, inc.AverageTrueRange.Last(0))
}

func (inc *Supertrend) GetSignal() types.Direction {
	return inc.tradeSignal
}

// GetDirection return the current trend
func (inc *Supertrend) Direction() types.Direction {
	return inc.trend
}

// LastSupertrendSupport return the current supertrend support
func (inc *Supertrend) LastSupertrendSupport() float64 {
	return inc.supportLine.Last(0)
}

// LastSupertrendResistance return the current supertrend resistance
func (inc *Supertrend) LastSupertrendResistance() float64 {
	return inc.resistanceLine.Last(0)
}

var _ types.SeriesExtend = &Supertrend{}

func (inc *Supertrend) PushK(k types.KLine) {
	if inc.EndTime != zeroTime && k.EndTime.Before(inc.EndTime) {
		return
	}

	inc.Update(k.GetHigh().Float64(), k.GetLow().Float64(), k.GetClose().Float64())
	inc.EndTime = k.EndTime.Time()
	inc.EmitUpdate(inc.Last(0))

}

func (inc *Supertrend) BindK(target KLineClosedEmitter, symbol string, interval types.Interval) {
	target.OnKLineClosed(types.KLineWith(symbol, interval, inc.PushK))
}

func (inc *Supertrend) LoadK(allKLines []types.KLine) {
	for _, k := range allKLines {
		inc.PushK(k)
	}
}

func (inc *Supertrend) CalculateAndUpdate(kLines []types.KLine) {
	for _, k := range kLines {
		if inc.EndTime != zeroTime && !k.EndTime.After(inc.EndTime) {
			continue
		}

		inc.PushK(k)
	}

	inc.EmitUpdate(inc.Last(0))
	inc.EndTime = kLines[len(kLines)-1].EndTime.Time()
}

func (inc *Supertrend) handleKLineWindowUpdate(interval types.Interval, window types.KLineWindow) {
	if inc.Interval != interval {
		return
	}

	inc.CalculateAndUpdate(window)
}

func (inc *Supertrend) Bind(updater KLineWindowUpdater) {
	updater.OnKLineWindowUpdate(inc.handleKLineWindowUpdate)
}
