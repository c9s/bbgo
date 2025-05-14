package klinedriver

import (
	"testing"
	"time"

	. "github.com/c9s/bbgo/pkg/testing/testhelper"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/stretchr/testify/assert"
)

type KLineCollector struct {
	KLines []*types.KLine
}

func (c *KLineCollector) EmitKLine(kline types.KLine) {
	c.KLines = append(c.KLines, &kline)
}
func (c *KLineCollector) EmitKLineClosed(kline types.KLine) {
	c.KLines = append(c.KLines, &kline)
}

// TestTickKLineDriver tests
// It also demonstrates how to use the driver in backtesting
func TestTickKLineDriver(t *testing.T) {
	symbol := "NOTEXIST"
	t.Run("TestTrades", func(t *testing.T) {
		tickStartTime, err := time.Parse(time.RFC3339, "2023-10-01T01:35:01Z")
		tickStartTimeTruncated := tickStartTime.Truncate(types.Interval1m.Duration())
		assert.NoError(t, err)
		driver := NewTickKLineDriver(symbol, time.Second*10)
		driver.AddInterval(types.Interval1m)
		kLineCollector := &KLineCollector{}
		driver.SetKLineEmitter(kLineCollector)
		driver.SetRunning(tickStartTime)
		for _, trade := range []types.Trade{
			{
				Symbol: symbol,
				Price:  Number(100),
				Time:   types.Time(tickStartTime.Add(5 * time.Second)),
			},
			{
				Symbol: symbol,
				Price:  Number(200),
				Time:   types.Time(tickStartTime.Add(15 * time.Second)),
			},
			{
				Symbol: symbol,
				Price:  Number(20),
				Time:   types.Time(tickStartTime.Add(30 * time.Second)),
			},
		} {
			driver.AddTrade(trade)
		}
		currentTickTime := tickStartTime.Add(30 * time.Second).Add(123 * time.Millisecond)
		driver.ProcessTick(currentTickTime)
		assert.Equal(t, 1, len(kLineCollector.KLines))
		kline := kLineCollector.KLines[0]
		assert.Equal(t, tickStartTimeTruncated, kline.StartTime.Time())
		assert.Equal(t, currentTickTime, kline.EndTime.Time())
		assert.Equal(t, Number(100), kline.Open)
		assert.Equal(t, Number(20), kline.Close)
		assert.Equal(t, Number(200), kline.High)
		assert.Equal(t, Number(20), kline.Low)
		assert.Equal(t, uint64(3), kline.NumberOfTrades)
		kLineCollector.KLines = nil
		driver.AddTrade(types.Trade{
			Symbol: symbol,
			Price:  Number(50),
			Time:   types.Time(tickStartTime.Add(55 * time.Second)),
		})
		currentTickTime = tickStartTime.Add(time.Minute).Add(123 * time.Millisecond)
		driver.ProcessTick(currentTickTime)
		// 1 closed kline, 1 open kline
		assert.Equal(t, 2, len(kLineCollector.KLines))
		// closed kline
		kline = kLineCollector.KLines[0]
		assert.Equal(t, tickStartTimeTruncated, kline.StartTime.Time())
		assert.Equal(t, tickStartTimeTruncated.Add(time.Minute).Add(-time.Millisecond), kline.EndTime.Time())
		assert.True(t, kline.Closed)
		assert.Equal(t, Number(100), kline.Open)
		assert.Equal(t, Number(50), kline.Close)
		assert.Equal(t, Number(200), kline.High)
		assert.Equal(t, Number(20), kline.Low)
		assert.Equal(t, uint64(4), kline.NumberOfTrades)
		// open kline
		kline = kLineCollector.KLines[1]
		assert.Equal(t, tickStartTimeTruncated.Add(time.Minute), kline.StartTime.Time())
		assert.Equal(t, currentTickTime, kline.EndTime.Time())
		assert.False(t, kline.Closed)
		assert.Equal(t, Number(50), kline.Open)
		assert.Equal(t, Number(50), kline.Close)
		assert.Equal(t, Number(50), kline.High)
		assert.Equal(t, Number(50), kline.Low)
		assert.Equal(t, uint64(0), kline.NumberOfTrades)
	})

	t.Run("TestNoTrades", func(t *testing.T) {
		tickStartTime, err := time.Parse(time.RFC3339, "2023-10-01T01:35:00Z")
		assert.NoError(t, err)
		driver := NewTickKLineDriver(symbol, time.Second*10)
		driver.AddInterval("1m")
		kLineCollector := &KLineCollector{}
		driver.SetKLineEmitter(kLineCollector)
		driver.SetRunning(tickStartTime)
		driver.ProcessTick(tickStartTime.Add(time.Minute))
		assert.Equal(t, 0, len(kLineCollector.KLines))
	})

	t.Run("TestMultipleEmptyIntervals", func(t *testing.T) {
		tickStartTime, err := time.Parse(time.RFC3339, "2023-10-01T01:35:00Z")
		assert.NoError(t, err)
		driver := NewTickKLineDriver(symbol, time.Second*10)
		driver.AddInterval("1m")
		kLineCollector := &KLineCollector{}
		driver.SetKLineEmitter(kLineCollector)
		driver.SetRunning(tickStartTime)
		driver.ProcessTick(tickStartTime.Add(time.Minute).Add(123 * time.Millisecond))
		assert.Equal(t, 0, len(kLineCollector.KLines))
		driver.ProcessTick(tickStartTime.Add(5 * time.Minute).Add(123 * time.Millisecond))
		assert.Equal(t, 0, len(kLineCollector.KLines))
	})

	t.Run("TestLastKLine", func(t *testing.T) {
		tickStartTime, err := time.Parse(time.RFC3339, "2023-10-01T01:35:01Z")
		tickStartTimeTruncated := tickStartTime.Truncate(types.Interval1m.Duration())
		assert.NoError(t, err)
		driver := NewTickKLineDriver(symbol, time.Second*10)
		driver.AddInterval(types.Interval1m)
		kLineCollector := &KLineCollector{}
		driver.SetKLineEmitter(kLineCollector)
		driver.SetRunning(tickStartTime)
		for _, trade := range []types.Trade{
			{
				Symbol: symbol,
				Price:  Number(100),
				Time:   types.Time(tickStartTime.Add(5 * time.Second)),
			},
			{
				Symbol: symbol,
				Price:  Number(200),
				Time:   types.Time(tickStartTime.Add(15 * time.Second)),
			},
			{
				Symbol: symbol,
				Price:  Number(20),
				Time:   types.Time(tickStartTime.Add(30 * time.Second)),
			},
			{
				Symbol: symbol,
				Price:  Number(50),
				Time:   types.Time(tickStartTime.Add(55 * time.Second)),
			},
		} {
			driver.AddTrade(trade)
		}
		currentTickTime := tickStartTime.Add(time.Minute).Add(123 * time.Millisecond)
		driver.ProcessTick(currentTickTime)
		assert.Equal(t, 2, len(kLineCollector.KLines))
		assert.True(t, kLineCollector.KLines[0].Closed)
		assert.False(t, kLineCollector.KLines[1].Closed)
		lastKLine := kLineCollector.KLines[0]
		kLineCollector.KLines = nil
		// no trades in the next 30 seconds
		currentTickTime = tickStartTime.Add(time.Minute).Add(30 * time.Second)
		driver.ProcessTick(currentTickTime)
		assert.Equal(t, 1, len(kLineCollector.KLines))
		kline := kLineCollector.KLines[0]
		assert.False(t, kline.Closed)
		assert.Equal(t, tickStartTimeTruncated.Add(time.Minute), kline.StartTime.Time())
		assert.Equal(t, currentTickTime, kline.EndTime.Time())
		assert.Equal(t, lastKLine.Close, kline.Open)
		assert.Equal(t, lastKLine.Close, kline.Close)
		assert.Equal(t, lastKLine.Close, kline.High)
		assert.Equal(t, lastKLine.Close, kline.Low)
		assert.Equal(t, uint64(0), kline.NumberOfTrades)
	})
}
