package klinedriver

import (
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
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

// FillGapKLines returns the kline slices that are used to fill the gap between the bucket and the next bucket.
// Note that it will also update the bucket's status including the start/end time and the kline.
func (bucket *Bucket) FillGapKLines(numShifts uint64) (kLines []*types.KLine) {
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
		bucket.KLine.StartTime = types.Time(bucket.StartTime.Time().UTC())
		bucket.KLine.EndTime = types.Time(bucket.EndTime.Time().UTC())
		// the opening kline
		kline := *bucket.KLine // copy
		kline.EndTime = types.Time(bucket.StartTime.Time().UTC().Add(time.Millisecond))
		kline.Closed = false
		kLines = append(kLines, &kline)
		// the closing kline
		kline = *bucket.KLine // copy again
		kline.EndTime = types.Time(bucket.EndTime.Time().UTC())
		kline.Closed = true
		kLines = append(kLines, &kline)
	}
	return kLines
}

func (bucket *Bucket) accumulateTrade(trade *types.Trade) {
	// expecting the trade time is in the same timezone as the kline
	// if you are about to use the trade time, make sure to convert the timezone properly
	// IMPORTANT: the trade should be treated as read-only here
	tradeTimeUTC := types.Time(trade.Time.Time().UTC())
	bucket.KLine.Exchange = trade.Exchange
	bucket.KLine.EndTime = tradeTimeUTC
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

// TODO: refactor by returning a slice of buckets of gap filling kline so we can remove FillGapKLines
func (bucket *Bucket) findNextBucket(currentTime types.Time) (nextBucket *Bucket, numShifts uint64) {
	nextBucket = &Bucket{
		StartTime: bucket.StartTime,
		EndTime:   bucket.EndTime,
		Interval:  bucket.Interval,
		KLine:     nil,
	}
	if currentTime.Before(nextBucket.StartTime.Time()) {
		return nextBucket, numShifts
	}
	intervalDuration := int64(nextBucket.Interval.Duration())
	numShifts = uint64(currentTime.Time().Sub(bucket.EndTime.Time())/nextBucket.Interval.Duration()) + 1
	nextBucket.StartTime = types.Time(nextBucket.StartTime.Time().Add(time.Duration(int64(numShifts) * intervalDuration)))
	nextBucket.EndTime = types.Time(nextBucket.EndTime.Time().Add(time.Duration(int64(numShifts) * intervalDuration)))
	return nextBucket, numShifts
}
