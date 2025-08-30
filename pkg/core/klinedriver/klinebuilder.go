package klinedriver

import (
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	log "github.com/sirupsen/logrus"
)

// KLineBuilder builds klines from trades and trades only
type KLineBuilder struct {
	symbol string

	// accBucketMap is a map from interval to accumulated kline of that interval
	lastBucketMap, accBucketMap map[types.Interval]*Bucket
	// buffer the klines that are not yet exposed
	klinesBuffer map[types.Interval][]*types.KLine
}

// NewKLineBuilder creates a new KLineBuilder.
// IMPORTANT: The klines built by this builder are in UTC timezone.
func NewKLineBuilder(symbol string) *KLineBuilder {
	return &KLineBuilder{
		symbol:        symbol,
		lastBucketMap: make(map[types.Interval]*Bucket),
		accBucketMap:  make(map[types.Interval]*Bucket),
		klinesBuffer:  make(map[types.Interval][]*types.KLine),
	}
}

// AddInterval adds an interval to the KLineBuilder.
// AddInterval should be called before any operations on the KLineBuilder.
func (kb *KLineBuilder) AddInterval(interval types.Interval, initTime types.Time) {
	kb.resetBucket(interval, initTime)
}

// AddTrade adds a trade to the KLineBuilder.
// The semantics of the AddTrade is to notify the current time to the KLineBuilder with the trade.
// The builder will accumulate the kline according to the trade time and update its state if needed.
// The builder assumes the trades are passed monotonically.
func (kb *KLineBuilder) AddTrade(trade types.Trade) {
	// ignore the trade of different symbol
	if trade.Symbol != kb.symbol {
		return
	}
	for interval, accBucket := range kb.accBucketMap {
		// trade is before the start time of the kline, ignore it
		if trade.Time.Before(accBucket.StartTime.Time()) {
			continue
		}
		// trade in the bucket, accumulate the kline and continue
		// need not to check if the kline is closed because the trade is in the bucket
		// accKLine must not be closed here
		if accBucket.Contains(trade.Time) {
			accBucket.accumulateTrade(&trade)
			continue
		}
		// trade is not in the current bucket, find the next containing bucket
		//                  trade
		// [ acc bucket )[--- * --)
		//
		//  or
		//	                     trade
		// [ acc bucket ) ... [--- * --)
		// close the current accumulating bucket
		accBucket.KLine.Closed = true
		// if the current accumulating bucket is not yet exposed, add it to the buffer
		if !accBucket.Exposed {
			kb.klinesBuffer[interval] = append(kb.klinesBuffer[interval], accBucket.KLine)
			accBucket.Exposed = true
		}
		// find the next containing bucket
		nextBucket, numShifts := accBucket.findNextBucket(trade.Time)
		// setup the last kline and update the accumulating kline
		lastBucket := *accBucket // make a copy
		kb.resetBucket(interval, nextBucket.StartTime)
		// fill the gap with empty trades
		fillingKLines := lastBucket.FillGapKLines(numShifts)
		kb.klinesBuffer[interval] = append(kb.klinesBuffer[interval], fillingKLines...)

		kb.lastBucketMap[interval] = &lastBucket
		accBucket.accumulateTrade(&trade)
	}
}

// Update updates the KLineBuilder with the given update time.
// The semantics of the Update is to notify the current timestamp to the KLineBuilder.
// Note that the start/end time of the klines are in UTC timezone.
// In addition to that, the end time will be (start time + interval) instead of (start time + interval - 1ms).
// The rationale is that we want to keep the logic as simple as possible and let the driver/caller
// to handle subtle adjustments, such as adjusting the end time to be 1ms before the next start time.
// Also, it will make it easier to write test for the KLineBuilder.
func (kb *KLineBuilder) Update(updateTime types.Time) (kLinesMap map[types.Interval][]*types.KLine) {
	kLinesMap = make(map[types.Interval][]*types.KLine)
	for interval, bufferedKLines := range kb.klinesBuffer {
		for _, klineRef := range bufferedKLines {
			kline := *klineRef
			kLinesMap[interval] = append(kLinesMap[interval], &kline)
		}
		kb.klinesBuffer[interval] = make([]*types.KLine, 0)
	}
	for interval, accBucket := range kb.accBucketMap {
		if updateTime.Before(accBucket.StartTime.Time()) {
			// update time is before the start time, just add kline
			kline := *accBucket.KLine
			kLinesMap[interval] = append(kLinesMap[interval], &kline)
			continue
		}

		// check if kline is available: there were trades or we have the last kline
		lastBucket, hasLast := kb.lastBucketMap[interval]
		if accBucket.KLine.NumberOfTrades == 0 && !hasLast {
			log.Debugf("kline not available for interval %s(%s) yet", kb.symbol, interval)
			continue
		}

		// At this point, either there were trades or we have the last kline
		if accBucket.KLine.NumberOfTrades == 0 {
			// no trade happened up to the updateTime
			// use the last kline to fill the gap
			accBucket.KLine.Open = lastBucket.KLine.Close
			accBucket.KLine.High = lastBucket.KLine.Close
			accBucket.KLine.Low = lastBucket.KLine.Close
			accBucket.KLine.Close = lastBucket.KLine.Close
			accBucket.KLine.LastTradeID = lastBucket.KLine.LastTradeID
			accBucket.KLine.Volume = fixedpoint.Zero
			accBucket.KLine.QuoteVolume = fixedpoint.Zero
			accBucket.KLine.TakerBuyBaseAssetVolume = fixedpoint.Zero
			accBucket.KLine.TakerBuyQuoteAssetVolume = fixedpoint.Zero
		}
		// two cases from here:
		// - the update time is in the accumulating bucket
		// - the update time is after the accumulating bucket
		//   - there might be multiple buckets in between

		// check if the update time is in the accumulating bucket
		if accBucket.Contains(updateTime) {
			// update time is in the accumulating bucket, just add kline
			accBucket.KLine.EndTime = types.Time(updateTime.Time().UTC())
			kline := *accBucket.KLine
			kLinesMap[interval] = append(kLinesMap[interval], &kline)
			continue
		}

		// update time is after the accumulating bucket
		// we should close the current accumulating bucket and add kline
		accBucket.KLine.Closed = true
		// we do not subtract 1ms from the end time here so that we can keep the logic simple for the builder.
		// we leave such adjustments to the caller of the builder.
		accBucket.KLine.EndTime = types.Time(accBucket.EndTime.Time().UTC())
		kline := *accBucket.KLine
		kLinesMap[interval] = append(kLinesMap[interval], &kline)
		accBucket.Exposed = true
		// find the next containing bucket and reset the accumulating bucket
		nextBucket, numShifts := accBucket.findNextBucket(updateTime)
		nextBucket = kb.resetBucket(interval, nextBucket.StartTime)
		// fill the gaps
		filledKLines := accBucket.FillGapKLines(numShifts)
		kLinesMap[interval] = append(kLinesMap[interval], filledKLines...)

		nextBucket.KLine.Open = accBucket.KLine.Close
		nextBucket.KLine.High = accBucket.KLine.Close
		nextBucket.KLine.Low = accBucket.KLine.Close
		nextBucket.KLine.Close = accBucket.KLine.Close
		nextBucket.KLine.LastTradeID = accBucket.KLine.LastTradeID
		kb.lastBucketMap[interval] = accBucket
		// add an opening kline
		nextBucket.KLine.EndTime = types.Time(updateTime.Time().UTC())
		nextBucket.KLine.Closed = false
		nextKLine := *nextBucket.KLine
		kLinesMap[interval] = append(kLinesMap[interval], &nextKLine)
	}
	return kLinesMap
}

func (kb *KLineBuilder) resetBucket(resetInterval types.Interval, resetTime types.Time) *Bucket {
	resetTimeUTC := resetTime.Time().UTC()
	newBucket := &Bucket{
		StartTime: resetTime,
		EndTime:   types.Time(resetTime.Time().Add(resetInterval.Duration())),
		Interval:  resetInterval,
		KLine: &types.KLine{
			Symbol:    kb.symbol,
			Interval:  resetInterval,
			StartTime: types.Time(resetTimeUTC),
			EndTime:   types.Time(resetTimeUTC.Add(time.Millisecond)),
			Closed:    false,
		},
	}
	kb.accBucketMap[resetInterval] = newBucket
	if lastBucket, hasLast := kb.lastBucketMap[resetInterval]; hasLast {
		newBucket.KLine.Exchange = lastBucket.KLine.Exchange
	}

	return newBucket
}
