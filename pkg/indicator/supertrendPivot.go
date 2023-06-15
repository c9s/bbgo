package indicator

import (
	"math"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/types"
)

// based on "Pivot Point Supertrend by LonesomeTheBlue" from tradingview

var logpst = logrus.WithField("indicator", "pivotSupertrend")

//go:generate callbackgen -type PivotSupertrend
type PivotSupertrend struct {
	types.SeriesBase
	types.IntervalWindow
	ATRMultiplier float64 `json:"atrMultiplier"`
	PivotWindow   int     `json:"pivotWindow"`

	AverageTrueRange *ATR // Value must be set when initialized in strategy

	PivotLow  *PivotLow  // Value must be set when initialized in strategy
	PivotHigh *PivotHigh // Value must be set when initialized in strategy

	trendPrices    floats.Slice // Tsl: value of the trend line (buy or sell)
	supportLine    floats.Slice // The support line in an uptrend (green)
	resistanceLine floats.Slice // The resistance line in a downtrend (red)

	closePrice             float64
	previousClosePrice     float64
	uptrendPrice           float64
	previousUptrendPrice   float64
	downtrendPrice         float64
	previousDowntrendPrice float64

	lastPp            float64
	src               float64 // center
	previousPivotHigh float64 // temp variable to save the last value
	previousPivotLow  float64 // temp variable to save the last value

	trend         types.Direction
	previousTrend types.Direction
	tradeSignal   types.Direction

	EndTime         time.Time
	UpdateCallbacks []func(value float64)
}

func (inc *PivotSupertrend) Last(i int) float64 {
	return inc.trendPrices.Last(i)
}

func (inc *PivotSupertrend) Index(i int) float64 {
	return inc.Last(i)
}

func (inc *PivotSupertrend) Length() int {
	return len(inc.trendPrices)
}

func (inc *PivotSupertrend) Update(highPrice, lowPrice, closePrice float64) {
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

	inc.previousPivotLow = inc.PivotLow.Last(0)
	inc.previousPivotHigh = inc.PivotHigh.Last(0)

	// Update High / Low pivots
	inc.PivotLow.Update(lowPrice)
	inc.PivotHigh.Update(highPrice)

	// Update ATR
	inc.AverageTrueRange.Update(highPrice, lowPrice, closePrice)

	// Update last prices
	inc.previousUptrendPrice = inc.uptrendPrice
	inc.previousDowntrendPrice = inc.downtrendPrice
	inc.previousClosePrice = inc.closePrice
	inc.previousTrend = inc.trend

	inc.closePrice = closePrice

	// Initialize lastPp as soon as pivots are made
	if inc.lastPp == 0 || math.IsNaN(inc.lastPp) {
		if inc.PivotHigh.Length() > 0 {
			inc.lastPp = inc.PivotHigh.Last(0)
		} else if inc.PivotLow.Length() > 0 {
			inc.lastPp = inc.PivotLow.Last(0)
		} else {
			inc.lastPp = math.NaN()
			return
		}
	}

	// Set lastPp to the latest pivotPoint (only changed when new pivot is found)
	if inc.PivotHigh.Last(0) != inc.previousPivotHigh {
		inc.lastPp = inc.PivotHigh.Last(0)
	} else if inc.PivotLow.Last(0) != inc.previousPivotLow {
		inc.lastPp = inc.PivotLow.Last(0)
	}

	// calculate the Center line using pivot points
	if inc.src == 0 || math.IsNaN(inc.src) {
		inc.src = inc.lastPp
	} else {
		// weighted calculation
		inc.src = (inc.src*2 + inc.lastPp) / 3
	}

	// Update uptrend
	inc.uptrendPrice = inc.src - inc.AverageTrueRange.Last(0)*inc.ATRMultiplier
	if inc.previousClosePrice > inc.previousUptrendPrice {
		inc.uptrendPrice = math.Max(inc.uptrendPrice, inc.previousUptrendPrice)
	}

	// Update downtrend
	inc.downtrendPrice = inc.src + inc.AverageTrueRange.Last(0)*inc.ATRMultiplier
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

	logpst.Debugf("Update pivot point supertrend result: closePrice: %v, uptrendPrice: %v, downtrendPrice: %v, trend: %v,"+
		" tradeSignal: %v, AverageTrueRange.Last(): %v", inc.closePrice, inc.uptrendPrice, inc.downtrendPrice,
		inc.trend, inc.tradeSignal, inc.AverageTrueRange.Last(0))
}

// GetSignal returns signal (Down, None or Up)
func (inc *PivotSupertrend) GetSignal() types.Direction {
	return inc.tradeSignal
}

// GetDirection return the current trend
func (inc *PivotSupertrend) Direction() types.Direction {
	return inc.trend
}

// LastSupertrendSupport return the current supertrend support value
func (inc *PivotSupertrend) LastSupertrendSupport() float64 {
	return inc.supportLine.Last(0)
}

// LastSupertrendResistance return the current supertrend resistance value
func (inc *PivotSupertrend) LastSupertrendResistance() float64 {
	return inc.resistanceLine.Last(0)
}

var _ types.SeriesExtend = &PivotSupertrend{}

func (inc *PivotSupertrend) PushK(k types.KLine) {
	if inc.EndTime != zeroTime && !k.EndTime.After(inc.EndTime) {
		return
	}

	inc.Update(k.GetHigh().Float64(), k.GetLow().Float64(), k.GetClose().Float64())
	inc.EndTime = k.EndTime.Time()
	inc.EmitUpdate(inc.Last(0))
}

func (inc *PivotSupertrend) BindK(target KLineClosedEmitter, symbol string, interval types.Interval) {
	target.OnKLineClosed(types.KLineWith(symbol, interval, inc.PushK))
}

func (inc *PivotSupertrend) LoadK(allKLines []types.KLine) {
	inc.SeriesBase.Series = inc
	for _, k := range allKLines {
		inc.PushK(k)
	}
}
