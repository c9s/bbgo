package klinedriver

import (
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	log "github.com/sirupsen/logrus"
)

type Bucket struct {
	// StartTime is inclusive, EndTime is exclusive
	// e.g. [StartTime, EndTime)
	StartTime, EndTime types.Time
	Interval           types.Interval
	KLine              *types.KLine
	Exposed            bool
}

// check if the currentTime is in [startTime, startTime + interval)
func (bucket *Bucket) Contains(currentTime types.Time) bool {
	if currentTime.Equal(bucket.StartTime.Time()) {
		return true
	}
	return currentTime.After(bucket.StartTime.Time()) && currentTime.Before(bucket.EndTime.Time())
}

// FillingGapKLines returns the kline slices that are used to fill the gap between the bucket and the next bucket.
// Note that it will also update the bucket's status including the start/end time and the kline.
func (bucket *Bucket) FillingGapKLines(numShifts uint64) (kLines []*types.KLine) {
	interval := bucket.Interval
	if numShifts > 1 {
		bucket.KLine.Open = bucket.KLine.Close
		bucket.KLine.High = bucket.KLine.Close
		bucket.KLine.Low = bucket.KLine.Close
		bucket.KLine.Volume = fixedpoint.Zero
		bucket.KLine.QuoteVolume = fixedpoint.Zero
		bucket.KLine.TakerBuyBaseAssetVolume = fixedpoint.Zero
		bucket.KLine.TakerBuyQuoteAssetVolume = fixedpoint.Zero
		bucket.KLine.NumberOfTrades = 0
	}
	// fill the gap with empty trades
	for i := uint64(1); i < numShifts; i++ {
		bucket.StartTime = types.Time(bucket.StartTime.Time().Add(interval.Duration()))
		bucket.EndTime = types.Time(bucket.EndTime.Time().Add(interval.Duration()))
		bucket.KLine.StartTime = bucket.StartTime
		bucket.KLine.EndTime = bucket.EndTime
		// the opening kline
		kline := *bucket.KLine // copy
		kline.EndTime = types.Time(bucket.StartTime.Time().Add(time.Millisecond))
		kline.Closed = false
		kLines = append(kLines, &kline)
		// the closing kline
		kline = *bucket.KLine // copy again
		kline.EndTime = bucket.EndTime
		kline.Closed = true
		kLines = append(kLines, &kline)
	}
	return
}

func (bucket *Bucket) accumulateTrade(trade *types.Trade) {
	// expecting the trade time is in the same timezone as the kline
	// if you are about to use the trade time, make sure to convert the timezone properly
	// IMPORTANT: the trade should be treated as read-only here
	tradeTimeInLocalTz := types.Time(trade.Time.Time().In(bucket.KLine.StartTime.Time().Location()))
	bucket.KLine.Exchange = trade.Exchange
	bucket.KLine.EndTime = tradeTimeInLocalTz
	bucket.KLine.Close = trade.Price
	bucket.KLine.High = fixedpoint.Max(bucket.KLine.High, trade.Price)
	if bucket.KLine.NumberOfTrades == 0 {
		bucket.KLine.Open = trade.Price
		bucket.KLine.Low = trade.Price
	} else {
		bucket.KLine.Low = fixedpoint.Min(bucket.KLine.Low, trade.Price)
	}
	bucket.KLine.Volume = bucket.KLine.Volume.Add(trade.Quantity)
	bucket.KLine.QuoteVolume = bucket.KLine.QuoteVolume.Add(trade.QuoteQuantity)
	bucket.KLine.NumberOfTrades++
	bucket.KLine.LastTradeID = trade.ID
	if trade.IsBuyer && !trade.IsMaker {
		bucket.KLine.TakerBuyBaseAssetVolume = bucket.KLine.TakerBuyBaseAssetVolume.Add(trade.Quantity)
		bucket.KLine.TakerBuyQuoteAssetVolume = bucket.KLine.TakerBuyQuoteAssetVolume.Add(trade.QuoteQuantity)
	}

}

func (bucket *Bucket) findNextBucket(currentTime types.Time) (nextBucket *Bucket, numShifts uint64) {
	nextBucket = &Bucket{
		StartTime: bucket.StartTime,
		EndTime:   bucket.EndTime,
		Interval:  bucket.Interval,
		KLine:     nil,
	}
	if currentTime.Before(nextBucket.StartTime.Time()) {
		return
	}
	for {
		if currentTime.Equal(nextBucket.EndTime.Time()) {
			nextBucket.StartTime = nextBucket.EndTime
			break
		}
		if currentTime.After(nextBucket.StartTime.Time()) && currentTime.Before(nextBucket.EndTime.Time()) {
			break
		}
		nextBucket.StartTime = nextBucket.EndTime
		nextBucket.EndTime = types.Time(nextBucket.StartTime.Time().Add(nextBucket.Interval.Duration()))
		numShifts++
	}
	return
}

// KLineBuilder builds klines from trades and trades only
type KLineBuilder struct {
	symbol string

	// accBucketMap is a map from interval to accumulated kline of that interval
	lastBucketMap, accBucketMap map[types.Interval]*Bucket
	// buffer the the klines that are not yet exposed
	klinesBuffer map[types.Interval][]*types.KLine
}

// NewKLineBuilder creates a new KLineBuilder.
// IMPORTANT: The klines built by this builder are in the same timezone as local time, not trade time.
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
	kb.resetKLine(interval, initTime)
}

// AddTrade adds a trade to the KLineBuilder.
// The semantics of the AddTrade is to notify the current time to the KLineBuilder with the trade.
// The builder will accumulate the kline according to the trade time and update its state if needed.
// The builder assumes the trades are passed monotonically.
func (kb *KLineBuilder) AddTrade(trade *types.Trade) {
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
			accBucket.accumulateTrade(trade)
			continue
		}
		// trade is not in the current bucket, find the next containing bucket
		//                  trade
		// ( acc bucket ](--- * --]
		//
		//  or
		//	                     trade
		// ( acc bucket ] ... (--- * --]
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
		kb.resetKLine(interval, nextBucket.StartTime)
		// fill the gap with empty trades
		fillingKLines := lastBucket.FillingGapKLines(numShifts)
		kb.klinesBuffer[interval] = append(kb.klinesBuffer[interval], fillingKLines...)

		kb.lastBucketMap[interval] = &lastBucket
		accBucket.accumulateTrade(trade)
	}
}

// Update updates the KLineBuilder with the given update time.
// The semantics of the Update is to notify the current timestamp to the KLineBuilder.
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
			accBucket.KLine.EndTime = updateTime
			kline := *accBucket.KLine
			kLinesMap[interval] = append(kLinesMap[interval], &kline)
			continue
		}

		// update time is after the accumulating bucket
		// we should close the current accumulating bucket and add kline
		accBucket.KLine.Closed = true
		accBucket.KLine.EndTime = accBucket.EndTime
		kline := *accBucket.KLine
		kLinesMap[interval] = append(kLinesMap[interval], &kline)
		accBucket.Exposed = true
		// find the next containing bucket and reset the accumulating bucket
		nextBucket, numShifts := accBucket.findNextBucket(updateTime)
		nextBucket = kb.resetKLine(interval, nextBucket.StartTime)
		// fill the gaps
		fillingKLines := accBucket.FillingGapKLines(numShifts)
		kLinesMap[interval] = append(kLinesMap[interval], fillingKLines...)

		nextBucket.KLine.Open = accBucket.KLine.Close
		nextBucket.KLine.High = accBucket.KLine.Close
		nextBucket.KLine.Low = accBucket.KLine.Close
		nextBucket.KLine.LastTradeID = accBucket.KLine.LastTradeID
		kb.lastBucketMap[interval] = accBucket
		// add an opening kline
		nextBucket.KLine.EndTime = updateTime
		nextBucket.KLine.Closed = false
		nextKLine := *nextBucket.KLine
		kLinesMap[interval] = append(kLinesMap[interval], &nextKLine)
	}
	return
}

func (kb *KLineBuilder) resetKLine(resetInterval types.Interval, resetTime types.Time) *Bucket {
	newBucket := &Bucket{
		StartTime: resetTime,
		EndTime:   types.Time(resetTime.Time().Add(resetInterval.Duration())),
		Interval:  resetInterval,
		KLine: &types.KLine{
			Symbol:    kb.symbol,
			Interval:  resetInterval,
			StartTime: resetTime,
			EndTime:   types.Time(resetTime.Time().Add(time.Millisecond)),
			Closed:    false,
		},
	}
	kb.accBucketMap[resetInterval] = newBucket
	lastBucket, hasLast := kb.lastBucketMap[resetInterval]
	if hasLast {
		newBucket.KLine.Exchange = lastBucket.KLine.Exchange
	}

	return newBucket
}
