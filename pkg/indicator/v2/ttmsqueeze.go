package indicatorv2

import (
	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/types"
)

type CompressionLevel int

const (
	CompressionLevelNone   CompressionLevel = iota // green dot
	CompressionLevelLow                            // black dot
	CompressionLevelMedium                         // red dot
	CompressionLevelHigh                           // orange dot
)

type MomentumDirection int

const (
	MomentumDirectionNeutral MomentumDirection = iota
	MomentumDirectionBullish
	MomentumDirectionBullishSlowing
	MomentumDirectionBearish
	MomentumDirectionBearishSlowing
)

type TTMSqueeze struct {
	CompressionLevel  CompressionLevel
	Momentum          float64
	MomentumDirection MomentumDirection
	Time              types.Time
}

// TTM Squeeze indicator stream
// Reference: https://www.tradingview.com/pine/?id=PUB%3B7e9cf40f672c4ab88ac70c580a327870
//
//go:generate callbackgen -type TTMSqueezeStream -output ttmsqueeze_callbacks.go
type TTMSqueezeStream struct {
	closeStream *PriceStream
	highStream  *PriceStream
	lowStream   *PriceStream
	boll        *BOLLStream
	keltner     *KeltnerStream

	window int

	// delta buffer for momentum linear regression calculation
	deltas floats.Slice

	// previous momentum for direction calculation
	prevMomentum float64

	updateCallbacks []func(squeeze TTMSqueeze)
}

func (ts *TTMSqueezeStream) handleKLine(kline types.KLine) {
	// since the callback is triggered in order,
	// we assume all upstream indicators including boll, keltner, high, low and close,
	// have been updated with the latest kline data when we receive the callback

	// not enough data, skip calculation
	if ts.boll.Length() == 0 {
		return
	}

	squeeze := TTMSqueeze{
		Time: kline.StartTime,
	}

	// Get BB bands
	bbUpper := ts.boll.UpBand.Last(0)
	bbLower := ts.boll.DownBand.Last(0)

	// Get KC bands (first = high/narrow mult=1.0, second = mid mult=1.5, third = low/wide mult=2.0)
	kcUpperHigh := ts.keltner.FirstUpperBand.Last(0)
	kcLowerHigh := ts.keltner.FirstLowerBand.Last(0)
	kcUpperMid := ts.keltner.SecondUpperBand.Last(0)
	kcLowerMid := ts.keltner.SecondLowerBand.Last(0)
	kcUpperLow := ts.keltner.ThirdUpperBand.Last(0)
	kcLowerLow := ts.keltner.ThirdLowerBand.Last(0)

	// Squeeze conditions (following the PineScript OR logic)
	// HighSqz = BB_lower >= KC_lower_high or BB_upper <= KC_upper_high
	// MidSqz = BB_lower >= KC_lower_mid or BB_upper <= KC_upper_mid
	// LowSqz = BB_lower >= KC_lower_low or BB_upper <= KC_upper_low
	highSqz := bbLower >= kcLowerHigh || bbUpper <= kcUpperHigh
	midSqz := bbLower >= kcLowerMid || bbUpper <= kcUpperMid
	lowSqz := bbLower >= kcLowerLow || bbUpper <= kcUpperLow

	// Determine compression level (priority: high > mid > low > none)
	if highSqz {
		squeeze.CompressionLevel = CompressionLevelHigh
	} else if midSqz {
		squeeze.CompressionLevel = CompressionLevelMedium
	} else if lowSqz {
		squeeze.CompressionLevel = CompressionLevelLow
	} else {
		squeeze.CompressionLevel = CompressionLevelNone
	}

	// Calculate momentum
	// mom = linreg(close - avg(avg(highest(high, length), lowest(low, length)), sma(close, length)), length, 0)
	// Step 1: Get highest high and lowest low over window
	highestHigh := ts.highStream.Highest(ts.window)
	lowestLow := ts.lowStream.Lowest(ts.window)

	// Step 2: Calculate donchian midline = avg(highest_high, lowest_low)
	donchianMid := (highestHigh + lowestLow) / 2.0

	// Step 3: Get SMA of close (from boll.SMA)
	smaClose := ts.boll.SMA.Last(0)

	// Step 4: Calculate average of donchian midline and SMA
	avgMid := (donchianMid + smaClose) / 2.0

	// Step 5: Calculate delta = close - avgMid
	closePrice := ts.closeStream.Last(0)
	delta := closePrice - avgMid

	// Step 6: Push delta and truncate to window size
	ts.deltas.Push(delta)
	// not enough data for momentum calculation, skip
	if ts.deltas.Length() < ts.window {
		return
	}
	ts.deltas.Truncate(ts.window + ts.window/2)

	// Step 7: Calculate linear regression of deltas
	squeeze.Momentum = types.Predict(&ts.deltas, ts.window, 0)

	// Calculate direction based on momentum change
	// mom > 0 and mom > mom[1] → bullish (aqua)
	// mom > 0 and mom <= mom[1] → bullish slowing (blue)
	// mom < 0 and mom < mom[1] → bearish (red)
	// mom < 0 and mom >= mom[1] → bearish slowing (yellow)
	if squeeze.Momentum > 0 {
		if squeeze.Momentum > ts.prevMomentum {
			squeeze.MomentumDirection = MomentumDirectionBullish
		} else {
			squeeze.MomentumDirection = MomentumDirectionBullishSlowing
		}
	} else if squeeze.Momentum < 0 {
		if squeeze.Momentum < ts.prevMomentum {
			squeeze.MomentumDirection = MomentumDirectionBearish
		} else {
			squeeze.MomentumDirection = MomentumDirectionBearishSlowing
		}
	} else {
		squeeze.MomentumDirection = MomentumDirectionNeutral
	}
	ts.prevMomentum = squeeze.Momentum

	ts.EmitUpdate(squeeze)
}

func NewTTMSqueezeStream(source KLineSubscription, window int) *TTMSqueezeStream {
	close := ClosePrices(source)
	high := HighPrices(source)
	low := LowPrices(source)
	boll := BOLL(close, window, 2)
	keltner := Keltner(source, window, window, 1, 1.5, 2.0)
	ttm := &TTMSqueezeStream{
		closeStream: close,
		highStream:  high,
		lowStream:   low,
		boll:        boll,
		keltner:     keltner,
		window:      window,
	}
	source.AddSubscriber(ttm.handleKLine)
	return ttm
}
