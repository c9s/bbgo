package klinedriver

import (
	"testing"
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
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

// TestTickDriver tests
// It also demonstrates how to use the driver in backtesting
func TestTickDriver(t *testing.T) {
	symbol := "NOTEXIST"
	t.Run("TestTrades", func(t *testing.T) {
		tickStartTime, _ := time.Parse(time.RFC3339, "2023-10-01T01:35:00Z")
		driver := NewTickKLineDriver(symbol, "10s")
		driver.Subscribe("1m")
		kLineCollector := &KLineCollector{}
		driver.SetKLineEmitter(kLineCollector)
		driver.SetRunning(tickStartTime)
		for _, trade := range []types.Trade{
			{
				Symbol: symbol,
				Price:  fixedpoint.NewFromFloat(100),
				Time:   types.Time(tickStartTime.Add(5 * time.Second)),
			},
			{
				Symbol: symbol,
				Price:  fixedpoint.NewFromFloat(200),
				Time:   types.Time(tickStartTime.Add(15 * time.Second)),
			},
			{
				Symbol: symbol,
				Price:  fixedpoint.NewFromFloat(20),
				Time:   types.Time(tickStartTime.Add(30 * time.Second)),
			},
		} {
			driver.AddTrade(trade)
		}
		driver.ProcessTick(tickStartTime.Add(30 * time.Second).Add(123 * time.Millisecond))
		assert.Equal(t, 1, len(kLineCollector.KLines))
		kline := kLineCollector.KLines[0]
		assert.Equal(t, fixedpoint.NewFromFloat(100), kline.Open)
		assert.Equal(t, fixedpoint.NewFromFloat(20), kline.Close)
		assert.Equal(t, fixedpoint.NewFromFloat(200), kline.High)
		assert.Equal(t, fixedpoint.NewFromFloat(20), kline.Low)
		assert.Equal(t, uint64(3), kline.NumberOfTrades)
		kLineCollector.KLines = nil
		driver.AddTrade(types.Trade{
			Symbol: symbol,
			Price:  fixedpoint.NewFromFloat(50),
			Time:   types.Time(tickStartTime.Add(55 * time.Second)),
		})
		driver.ProcessTick(tickStartTime.Add(time.Minute).Add(123 * time.Millisecond))
		// 1 closed kline, 1 open kline
		assert.Equal(t, 2, len(kLineCollector.KLines))
		// closed kline
		kline = kLineCollector.KLines[0]
		assert.True(t, kline.Closed)
		assert.Equal(t, fixedpoint.NewFromFloat(100), kline.Open)
		assert.Equal(t, fixedpoint.NewFromFloat(50), kline.Close)
		assert.Equal(t, fixedpoint.NewFromFloat(200), kline.High)
		assert.Equal(t, fixedpoint.NewFromFloat(20), kline.Low)
		assert.Equal(t, uint64(4), kline.NumberOfTrades)
		// open kline
		kline = kLineCollector.KLines[1]
		assert.False(t, kline.Closed)
		assert.Equal(t, fixedpoint.NewFromFloat(50), kline.Open)
		assert.Equal(t, fixedpoint.NewFromFloat(50), kline.Close)
		assert.Equal(t, fixedpoint.NewFromFloat(50), kline.High)
		assert.Equal(t, fixedpoint.NewFromFloat(50), kline.Low)
		assert.Equal(t, uint64(0), kline.NumberOfTrades)
	})

	t.Run("TestNoTrades", func(t *testing.T) {
		tickStartTime, _ := time.Parse(time.RFC3339, "2023-10-01T01:35:00Z")
		driver := NewTickKLineDriver(symbol, "10s")
		driver.Subscribe("1m")
		kLineCollector := &KLineCollector{}
		driver.SetKLineEmitter(kLineCollector)
		driver.SetRunning(tickStartTime)
		driver.ProcessTick(tickStartTime.Add(time.Minute))
		assert.Equal(t, 0, len(kLineCollector.KLines))
	})

	t.Run("TestMultipleEmptyIntervals", func(t *testing.T) {
		tickStartTime, _ := time.Parse(time.RFC3339, "2023-10-01T01:35:00Z")
		driver := NewTickKLineDriver(symbol, "10s")
		driver.Subscribe("1m")
		kLineCollector := &KLineCollector{}
		driver.SetKLineEmitter(kLineCollector)
		driver.SetRunning(tickStartTime)
		driver.ProcessTick(tickStartTime.Add(time.Minute).Add(123 * time.Millisecond))
		assert.Equal(t, 0, len(kLineCollector.KLines))
		driver.ProcessTick(tickStartTime.Add(5 * time.Minute).Add(123 * time.Millisecond))
		assert.Equal(t, 0, len(kLineCollector.KLines))
	})

	t.Run("TestLastKLine", func(t *testing.T) {
		tickStartTime, _ := time.Parse(time.RFC3339, "2023-10-01T01:35:00Z")
		driver := NewTickKLineDriver(symbol, "10s")
		driver.Subscribe("1m")
		kLineCollector := &KLineCollector{}
		driver.SetKLineEmitter(kLineCollector)
		driver.SetRunning(tickStartTime)
		for _, trade := range []types.Trade{
			{
				Symbol: symbol,
				Price:  fixedpoint.NewFromFloat(100),
				Time:   types.Time(tickStartTime.Add(5 * time.Second)),
			},
			{
				Symbol: symbol,
				Price:  fixedpoint.NewFromFloat(200),
				Time:   types.Time(tickStartTime.Add(15 * time.Second)),
			},
			{
				Symbol: symbol,
				Price:  fixedpoint.NewFromFloat(20),
				Time:   types.Time(tickStartTime.Add(30 * time.Second)),
			},
			{
				Symbol: symbol,
				Price:  fixedpoint.NewFromFloat(50),
				Time:   types.Time(tickStartTime.Add(55 * time.Second)),
			},
		} {
			driver.AddTrade(trade)
		}
		driver.ProcessTick(tickStartTime.Add(time.Minute).Add(123 * time.Millisecond))
		assert.Equal(t, 2, len(kLineCollector.KLines))
		assert.True(t, kLineCollector.KLines[0].Closed)
		assert.False(t, kLineCollector.KLines[1].Closed)
		lastKLine := kLineCollector.KLines[0]
		kLineCollector.KLines = nil
		// no trades in the next 30 seconds
		driver.ProcessTick(tickStartTime.Add(time.Minute).Add(30 * time.Second))
		assert.Equal(t, 1, len(kLineCollector.KLines))
		kline := kLineCollector.KLines[0]
		assert.False(t, kline.Closed)
		assert.Equal(t, lastKLine.Close, kline.Open)
		assert.Equal(t, lastKLine.Close, kline.Close)
		assert.Equal(t, lastKLine.Close, kline.High)
		assert.Equal(t, lastKLine.Close, kline.Low)
		assert.Equal(t, uint64(0), kline.NumberOfTrades)
	})
}
