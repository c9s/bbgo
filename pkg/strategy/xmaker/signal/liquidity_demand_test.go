package signal

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	indicatorv2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

// createBuyDemandKLine creates a candle with buy demand
// Buy demand is concentrated (small range above open) indicating strong buying pressure
func createBuyDemandKLine(open, close float64, t time.Time) types.KLine {
	// For green candle with high buy demand:
	// - Small range above open (high - open should be small) -> high buyDemand
	// - Large range below open (open - low should be large) -> low sellDemand
	// - This creates positive net demand
	return types.KLine{
		Open:      fixedpoint.NewFromFloat(open),
		Close:     fixedpoint.NewFromFloat(close),
		High:      fixedpoint.NewFromFloat(close + 0.5), // Small range above open for concentrated buying
		Low:       fixedpoint.NewFromFloat(open - 5.0),  // Large range below open for distributed selling
		Volume:    fixedpoint.NewFromFloat(10000.0),
		StartTime: types.Time(t),
		EndTime:   types.Time(t.Add(time.Minute)),
	}
}

// createSellDemandKLine creates a candle with high sell demand
// Sell demand is concentrated (small range below open) indicating strong selling pressure
func createSellDemandKLine(open, close float64, t time.Time) types.KLine {
	// For red candle with high sell demand:
	// - Large range above open (high - open should be large) -> low buyDemand
	// - Small range below open (open - low should be small) -> high sellDemand
	// - This creates negative net demand
	return types.KLine{
		Open:      fixedpoint.NewFromFloat(open),
		Close:     fixedpoint.NewFromFloat(close),
		High:      fixedpoint.NewFromFloat(open + 5.0),  // Large range above open for distributed buying
		Low:       fixedpoint.NewFromFloat(close - 0.5), // Small range below open for concentrated selling
		Volume:    fixedpoint.NewFromFloat(10000.0),
		StartTime: types.Time(t),
		EndTime:   types.Time(t.Add(time.Minute)),
	}
}

// newTestLiquidityDemandSignal creates a signal with test-friendly configuration
// and returns both the signal and the klineStream for feeding data
func newTestLiquidityDemandSignal() (*LiquidityDemandSignal, *indicatorv2.KLineStream) {
	iw := types.IntervalWindow{
		Interval: types.Interval1m,
		Window:   2,
	}

	signal := &LiquidityDemandSignal{
		IntervalWindow:         iw,
		Threshold:              0.5, // Threshold for normalized signal value
		WarmUpSampleCount:      5,   // Reduced for testing
		StatsUpdateSampleCount: 10,  // Reduced for testing
	}

	// Create the underlying indicator using the public constructor
	klineStream := &indicatorv2.KLineStream{}
	sellMA := indicatorv2.SMA(types.NewFloat64Series(), iw.Window)
	buyMA := indicatorv2.SMA(types.NewFloat64Series(), iw.Window)
	signal.indicator = indicatorv2.LiquidityDemand(klineStream, sellMA, buyMA)
	signal.statsUpdateSampleCountf = float64(signal.StatsUpdateSampleCount)

	return signal, klineStream
}

// feedKLines feeds klines to the klineStream which will trigger the indicator updates
func feedKLines(klineStream *indicatorv2.KLineStream, klines []types.KLine) {
	for _, kline := range klines {
		klineStream.EmitUpdate(kline)
	}
}

func TestLiquidityDemandSignal_SixConsecutiveGreenCandles(t *testing.T) {
	signal, klineStream := newTestLiquidityDemandSignal()
	ctx := context.Background()

	// Create 6 consecutive green candles with INCREASING liquidity demand (strengthening bullish momentum)
	baseTime := time.Now()
	var klines []types.KLine
	basePrice := 100.0
	for i := 0; i < 6; i++ {
		open := basePrice + float64(i)*10.0
		close := open + 8.0 // Green candle (close > open)
		// For increasing buy demand: progressively smaller upward wick, progressively larger downward wick
		highAboveOpen := 0.5 - float64(i)*0.05 // Decreasing range above open (more concentrated buying over time)
		lowBelowOpen := 15.0 + float64(i)*2.0  // Increasing range below open (more distributed selling over time)
		kline := types.KLine{
			Open:      fixedpoint.NewFromFloat(open),
			Close:     fixedpoint.NewFromFloat(close),
			High:      fixedpoint.NewFromFloat(open + highAboveOpen),
			Low:       fixedpoint.NewFromFloat(open - lowBelowOpen),
			Volume:    fixedpoint.NewFromFloat(50000.0), // Increased volume for stronger signal
			StartTime: types.Time(baseTime.Add(time.Duration(i) * time.Minute)),
			EndTime:   types.Time(baseTime.Add(time.Duration(i+1) * time.Minute)),
		}
		klines = append(klines, kline)
	}

	// Feed the klines to the indicator
	feedKLines(klineStream, klines)

	// Calculate signal after warmup
	sig, err := signal.CalculateSignal(ctx)
	assert.NoError(t, err)

	// For consecutive green candles with increasing liquidity demand, we expect:
	// - High and increasing buy demand (volume concentrated in progressively smaller upward price range)
	// - Low sell demand (volume spread over larger downward price range)
	// - Positive and increasing net demand
	// - Positive signal after normalization (since last value > mean)
	t.Logf("Signal value after 6 green candles: %.4f", sig)

	// The signal should be positive, indicating bullish sentiment
	// Since consecutive green candles show strengthening buying pressure
	assert.True(t, sig > 0, "Expected positive signal for consecutive green candles, got %.4f", sig)
}

func TestLiquidityDemandSignal_SixConsecutiveRedCandles(t *testing.T) {
	signal, klineStream := newTestLiquidityDemandSignal()
	ctx := context.Background()

	// Create 6 consecutive red candles with INCREASING sell demand (strengthening bearish momentum)
	baseTime := time.Now()
	var klines []types.KLine
	basePrice := 100.0
	for i := 0; i < 6; i++ {
		open := basePrice - float64(i)*10.0
		close := open - 8.0 // Red candle (close < open)
		// For increasing sell demand: progressively larger upward wick, progressively smaller downward wick
		highAboveOpen := 15.0 + float64(i)*2.0 // Increasing range above open (more distributed buying over time)
		lowBelowOpen := 0.5 - float64(i)*0.05  // Decreasing range below open (more concentrated selling over time)
		kline := types.KLine{
			Open:      fixedpoint.NewFromFloat(open),
			Close:     fixedpoint.NewFromFloat(close),
			High:      fixedpoint.NewFromFloat(open + highAboveOpen),
			Low:       fixedpoint.NewFromFloat(open - lowBelowOpen),
			Volume:    fixedpoint.NewFromFloat(50000.0), // Increased volume for stronger signal
			StartTime: types.Time(baseTime.Add(time.Duration(i) * time.Minute)),
			EndTime:   types.Time(baseTime.Add(time.Duration(i+1) * time.Minute)),
		}
		klines = append(klines, kline)
	}

	// Feed the klines to the indicator
	feedKLines(klineStream, klines)

	// Calculate signal after warmup
	sig, err := signal.CalculateSignal(ctx)
	assert.NoError(t, err)

	// For consecutive red candles with increasing sell demand, we expect:
	// - Low buy demand (volume spread over larger upward price range)
	// - High and increasing sell demand (volume concentrated in progressively smaller downward price range)
	// - Negative and increasingly negative net demand
	// - Negative signal after normalization (since last value < mean, more negative)
	t.Logf("Signal value after 6 red candles: %.4f", sig)

	// The signal should be negative, indicating bearish sentiment
	// Since consecutive red candles show strengthening selling pressure
	assert.True(t, sig < 0, "Expected negative signal for consecutive red candles, got %.4f", sig)
}

func TestLiquidityDemandSignal_ThreeGreenThenThreeRed(t *testing.T) {
	signal, klineStream := newTestLiquidityDemandSignal()
	ctx := context.Background()

	// Create 3 green candles followed by 3 red candles
	baseTime := time.Now()
	var klines []types.KLine
	basePrice := 100.0

	// First 3 green candles
	for i := 0; i < 3; i++ {
		open := basePrice + float64(i)*5.0
		close := open + 4.0 // Green candle
		kline := createBuyDemandKLine(open, close, baseTime.Add(time.Duration(i)*time.Minute))
		klines = append(klines, kline)
	}

	// Next 3 red candles
	for i := 3; i < 6; i++ {
		open := basePrice + float64(2-i)*5.0 + 19.0 // Start from the high
		close := open - 4.0                         // Red candle
		kline := createSellDemandKLine(open, close, baseTime.Add(time.Duration(i)*time.Minute))
		klines = append(klines, kline)
	}

	// Feed the klines to the indicator
	feedKLines(klineStream, klines)

	// Calculate signal after the reversal
	sig, err := signal.CalculateSignal(ctx)
	assert.NoError(t, err)

	// For 3 green then 3 red candles:
	// - The signal uses a moving average, so recent red candles will dominate
	// - We expect the signal to shift toward negative as the recent trend is bearish
	// - The exact value depends on the MA window, but it should reflect the reversal
	t.Logf("Signal value after 3 green + 3 red candles: %.4f", sig)

	// The signal should be negative because the most recent candles (red) have more weight
	// in the moving average calculation
	assert.True(t, sig < 0, "Expected negative signal after reversal to red candles, got %.4f", sig)
}

func TestLiquidityDemandSignal_WarmupPeriod(t *testing.T) {
	signal, klineStream := newTestLiquidityDemandSignal()
	ctx := context.Background()

	// Feed only 2 candles (less than WarmUpSampleCount of 5)
	baseTime := time.Now()
	klines := []types.KLine{
		createBuyDemandKLine(100.0, 104.0, baseTime),
		createBuyDemandKLine(105.0, 109.0, baseTime.Add(time.Minute)),
	}

	feedKLines(klineStream, klines)

	// Calculate signal during warmup
	sig, err := signal.CalculateSignal(ctx)
	assert.NoError(t, err)

	// During warmup period, signal should be zero
	assert.Equal(t, 0.0, sig, "Expected zero signal during warmup period")
}

func TestLiquidityDemandSignal_ExtremeValues(t *testing.T) {
	t.Run("NaN", func(t *testing.T) {
		signal, klineStream := newTestLiquidityDemandSignal()
		signal.Threshold = 0.0
		ctx := context.Background()

		// Create 6 identical candles to produce identical liquidity demand values
		// This will result in std = 0 and last = mean, causing NaN in score calculation
		baseTime := time.Now()
		var klines []types.KLine
		for i := 0; i < 6; i++ {
			// All candles identical: same OHLC values
			kline := types.KLine{
				Open:      fixedpoint.NewFromFloat(100.0),
				Close:     fixedpoint.NewFromFloat(100.0),
				High:      fixedpoint.NewFromFloat(100.0),
				Low:       fixedpoint.NewFromFloat(100.0),
				Volume:    fixedpoint.NewFromFloat(1000.0),
				StartTime: types.Time(baseTime.Add(time.Duration(i) * time.Minute)),
				EndTime:   types.Time(baseTime.Add(time.Duration(i+1) * time.Minute)),
			}
			klines = append(klines, kline)
		}

		feedKLines(klineStream, klines)

		// Calculate signal - should handle NaN gracefully
		sig, err := signal.CalculateSignal(ctx)
		assert.NoError(t, err)

		// When raw score is NaN (0/0), signal should be converted to 0
		assert.Equal(t, 0.0, sig, "Expected zero signal when raw score is NaN")
		t.Logf("Signal value for NaN case: %.4f", sig)
	})

	t.Run("PositiveInf", func(t *testing.T) {
		signal, _ := newTestLiquidityDemandSignal()
		signal.Threshold = 0.0
		ctx := context.Background()

		// Directly push identical liquidity demand values to the indicator
		// This produces std = 0, and then push a much larger value
		// causing score to be +Inf (positive_value / 0)
		for i := 0; i < 20; i++ {
			signal.indicator.Push(100.0)
		}
		// Push a significantly higher value
		signal.indicator.Push(10000.0)

		// Calculate signal - should handle +Inf gracefully
		sig, err := signal.CalculateSignal(ctx)
		assert.NoError(t, err)

		// When raw score is +Inf (due to std ≈ 0 and last > mean), tanh(+Inf) = 1, scaled to 2
		// The signal should be very close to 2.0
		assert.Equal(t, 2.0, sig, "Expected signal of 2.0 when raw score is +Inf")
		t.Logf("Signal value for +Inf case: %.4f", sig)
	})

	t.Run("NegativeInf", func(t *testing.T) {
		signal, _ := newTestLiquidityDemandSignal()
		signal.Threshold = 0.0
		ctx := context.Background()

		// Directly push identical liquidity demand values to the indicator
		// This produces std = 0, and then push a much smaller value
		// causing score to be -Inf (negative_value / 0)
		for i := 0; i < 20; i++ {
			signal.indicator.Push(100.0)
		}
		// Push a significantly lower value
		signal.indicator.Push(-10000.0)

		// Calculate signal - should handle -Inf gracefully
		sig, err := signal.CalculateSignal(ctx)
		assert.NoError(t, err)

		// When raw score is -Inf (due to std ≈ 0 and last < mean), tanh(-Inf) = -1, scaled to -2
		// The signal should be very close to -2.0
		assert.Equal(t, -2.0, sig, "Expected signal of -2.0 when raw score is -Inf")
		t.Logf("Signal value for -Inf case: %.4f", sig)
	})
}

func TestLiquidityDemandSignal_BelowThreshold(t *testing.T) {
	signal, klineStream := newTestLiquidityDemandSignal()
	signal.Threshold = 1000000.0 // Very high threshold
	ctx := context.Background()

	// Create candles with low liquidity demand
	baseTime := time.Now()
	var klines []types.KLine
	for i := 0; i < 6; i++ {
		// Create candles with wide price ranges (low demand concentration)
		kline := types.KLine{
			Open:      fixedpoint.NewFromFloat(100.0),
			Close:     fixedpoint.NewFromFloat(101.0),
			High:      fixedpoint.NewFromFloat(110.0),  // Wide range
			Low:       fixedpoint.NewFromFloat(90.0),   // Wide range
			Volume:    fixedpoint.NewFromFloat(1000.0), // Low volume
			StartTime: types.Time(baseTime.Add(time.Duration(i) * time.Minute)),
			EndTime:   types.Time(baseTime.Add(time.Duration(i+1) * time.Minute)),
		}
		klines = append(klines, kline)
	}

	feedKLines(klineStream, klines)

	// Calculate signal
	sig, err := signal.CalculateSignal(ctx)
	assert.NoError(t, err)

	// When liquidity demand is below threshold, signal should be zero
	assert.Equal(t, 0.0, sig, "Expected zero signal when below threshold")
}
