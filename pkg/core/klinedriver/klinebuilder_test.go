package klinedriver

import (
	"testing"
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/stretchr/testify/assert"
)

func TestKLineBuilder(t *testing.T) {
	symbol := "BTCUSDT"
	t.Run("WithTrades", func(t *testing.T) {
		builder := NewKLineBuilder(symbol)
		startTime := types.Time(time.Now())
		builder.AddInterval("1m", startTime)
		builder.AddInterval("5m", startTime)
		trades := []types.Trade{
			// trades that are before the start time should be ignored
			{
				Symbol: symbol,
				Price:  fixedpoint.NewFromFloat(5.0),
				Time:   types.Time(startTime.Time().Add(-5 * time.Second)),
			},
			{
				Symbol: symbol,
				Price:  fixedpoint.NewFromFloat(100),
				Time:   types.Time(startTime.Time().Add(5 * time.Second)),
			},
			{
				Symbol: symbol,
				Price:  fixedpoint.NewFromFloat(200),
				Time:   types.Time(startTime.Time().Add(15 * time.Second)),
			},
			{
				Symbol: symbol,
				Price:  fixedpoint.NewFromFloat(20),
				Time:   types.Time(startTime.Time().Add(30 * time.Second)),
			},
			{
				Symbol: symbol,
				Price:  fixedpoint.NewFromFloat(50),
				Time:   types.Time(startTime.Time().Add(55 * time.Second)),
			},
		}
		for _, trade := range trades {
			builder.AddTrade(&trade)
		}
		updateTime := types.Time(startTime.Time().Add(1 * time.Minute))
		kLines := builder.Update(updateTime)
		kline1m, found1m := kLines["1m"]
		kline5m, found5m := kLines["5m"]
		assert.True(t, found1m && found5m)
		assert.Equal(t, true, kline1m[0].Closed)
		assert.Equal(t, false, kline5m[0].Closed)
		for _, kline := range kLines {
			assert.Equal(t, fixedpoint.NewFromFloat(100), kline[0].Open)
			assert.Equal(t, fixedpoint.NewFromFloat(50), kline[0].Close)
			assert.Equal(t, fixedpoint.NewFromFloat(200), kline[0].High)
			assert.Equal(t, fixedpoint.NewFromFloat(20), kline[0].Low)
			assert.Equal(t, uint64(4), kline[0].NumberOfTrades)
		}
	})

	t.Run("MultipleEmptyIntervals", func(t *testing.T) {
		builder := NewKLineBuilder(symbol)
		startTime := types.Time(time.Now())
		builder.AddInterval("1m", startTime)
		trades := []types.Trade{
			{
				Symbol: symbol,
				Price:  fixedpoint.NewFromFloat(100),
				Time:   types.Time(startTime.Time().Add(5 * time.Second)),
			},
			{
				Symbol: symbol,
				Price:  fixedpoint.NewFromFloat(200),
				Time:   types.Time(startTime.Time().Add(15 * time.Second)),
			},
			{
				Symbol: symbol,
				Price:  fixedpoint.NewFromFloat(20),
				Time:   types.Time(startTime.Time().Add(30 * time.Second)),
			},
			{
				Symbol: symbol,
				Price:  fixedpoint.NewFromFloat(50),
				Time:   types.Time(startTime.Time().Add(55 * time.Second)),
			},
		}
		for _, trade := range trades {
			builder.AddTrade(&trade)
		}
		// 1 interval passed
		updateTime := types.Time(startTime.Time().Add(1 * time.Minute))
		oldkLines := builder.Update(updateTime)
		assert.Equal(t, 1, len(oldkLines))
		oldKLine1m, found1m := oldkLines["1m"]
		assert.True(t, found1m)
		assert.True(t, oldKLine1m[0].Closed)
		assert.Equal(t, uint64(len(trades)), oldKLine1m[0].NumberOfTrades)
		// two empty intervals
		updateTime = types.Time(updateTime.Time().Add(2 * time.Minute))
		kLines := builder.Update(updateTime)
		assert.Equal(t, 1, len(kLines))
		kline1m, found1m := kLines["1m"]
		assert.True(t, found1m)
		assert.True(t, kline1m[0].Closed)
		assert.Equal(t, oldKLine1m[0].Close, kline1m[0].Open)
		assert.Equal(t, oldKLine1m[0].Close, kline1m[0].High)
		assert.Equal(t, oldKLine1m[0].Close, kline1m[0].Low)
		assert.Equal(t, oldKLine1m[0].Close, kline1m[0].Close)
		assert.Equal(t, uint64(0), kline1m[0].NumberOfTrades)
	})

	t.Run("MultipleNonEmptyIntervals", func(t *testing.T) {
		builder := NewKLineBuilder(symbol)
		startTime := types.Time(time.Now())
		builder.AddInterval("1m", startTime)
		for _, trade := range []types.Trade{
			{
				Symbol: symbol,
				Price:  fixedpoint.NewFromFloat(100),
				Time:   types.Time(startTime.Time().Add(5 * time.Second)),
			},
			{
				Symbol: symbol,
				Price:  fixedpoint.NewFromFloat(200),
				Time:   types.Time(startTime.Time().Add(15 * time.Second)),
			},
			{
				Symbol: symbol,
				Price:  fixedpoint.NewFromFloat(20),
				Time:   types.Time(startTime.Time().Add(30 * time.Second)),
			},
			{
				Symbol: symbol,
				Price:  fixedpoint.NewFromFloat(50),
				Time:   types.Time(startTime.Time().Add(55 * time.Second)),
			},
		} {
			builder.AddTrade(&trade)
		}
		// 1 interval passed
		updateTime := types.Time(startTime.Time().Add(1 * time.Minute))
		builder.Update(updateTime)
		// more trades
		for _, trade := range []types.Trade{
			{
				Symbol: symbol,
				Price:  fixedpoint.NewFromFloat(100),
				Time:   types.Time(updateTime.Time().Add(10 * time.Second)),
			},
			{
				Symbol: symbol,
				Price:  fixedpoint.NewFromFloat(200),
				Time:   types.Time(updateTime.Time().Add(23 * time.Second)),
			},
			{
				Symbol: symbol,
				Price:  fixedpoint.NewFromFloat(51),
				Time:   types.Time(updateTime.Time().Add(59 * time.Second)),
			},
		} {
			builder.AddTrade(&trade)
		}
		// 2 intervals passed
		updateTime = types.Time(updateTime.Time().Add(1 * time.Minute))
		kLines := builder.Update(updateTime)
		assert.Equal(t, 1, len(kLines))
		kline1m, found1m := kLines["1m"]
		assert.True(t, found1m)
		assert.True(t, kline1m[0].Closed)
		assert.Equal(t, fixedpoint.NewFromFloat(100), kline1m[0].Open)
		assert.Equal(t, fixedpoint.NewFromFloat(200), kline1m[0].High)
		assert.Equal(t, fixedpoint.NewFromFloat(51), kline1m[0].Low)
		assert.Equal(t, fixedpoint.NewFromFloat(51), kline1m[0].Close)
		assert.Equal(t, uint64(3), kline1m[0].NumberOfTrades)
	})

	t.Run("NoTrades", func(t *testing.T) {
		builder := NewKLineBuilder(symbol)
		startTime := types.Time(time.Now())
		builder.AddInterval("1m", startTime)
		builder.AddInterval("5m", startTime)
		// 3 minutes later
		resetTime := types.Time(startTime.Time().Add(3 * time.Minute))
		kLines := builder.Update(resetTime)
		assert.Equal(t, 0, len(kLines))
		// 2 more minutes later
		resetTime = types.Time(resetTime.Time().Add(2 * time.Minute))
		kLines = builder.Update(resetTime)
		assert.Equal(t, 0, len(kLines))
	})

	t.Run("LastKLine", func(t *testing.T) {
		builder := NewKLineBuilder(symbol)
		startTime := types.Time(time.Now())
		builder.AddInterval("1m", startTime)
		builder.AddInterval("5m", startTime)
		trades := []types.Trade{
			{
				Symbol: symbol,
				Price:  fixedpoint.NewFromFloat(100),
				Time:   types.Time(startTime.Time().Add(5 * time.Second)),
			},
			{
				Symbol: symbol,
				Price:  fixedpoint.NewFromFloat(200),
				Time:   types.Time(startTime.Time().Add(15 * time.Second)),
			},
			{
				Symbol: symbol,
				Price:  fixedpoint.NewFromFloat(20),
				Time:   types.Time(startTime.Time().Add(30 * time.Second)),
			},
			{
				Symbol: symbol,
				Price:  fixedpoint.NewFromFloat(50),
				Time:   types.Time(startTime.Time().Add(55 * time.Second)),
			},
		}
		for _, trade := range trades {
			builder.AddTrade(&trade)
		}
		updateTime := types.Time(startTime.Time().Add(5 * time.Minute))
		lastKLines := builder.Update(updateTime)
		updateTime2 := types.Time(updateTime.Time().Add(5 * time.Minute))
		newKLines := builder.Update(updateTime2)
		assert.Equal(t, len(lastKLines), len(newKLines))
		for interval, lastKLine := range lastKLines {
			newKLine, ok := newKLines[interval]
			if !ok {
				assert.Fail(t, "kline not found")
			}
			assert.Equal(t, newKLine[0].StartTime, updateTime)
			assert.Equal(t, lastKLine[0].Close, newKLine[0].Open)
			assert.Equal(t, lastKLine[0].Close, newKLine[0].High)
			assert.Equal(t, lastKLine[0].Close, newKLine[0].Low)
			assert.Equal(t, lastKLine[0].Close, newKLine[0].Close)
			assert.True(t, newKLine[0].Volume.IsZero())
			assert.True(t, newKLine[0].QuoteVolume.IsZero())
			assert.Equal(t, uint64(0), newKLine[0].NumberOfTrades)
		}
	})
}
