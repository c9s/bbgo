package v1

import (
	"fmt"
	"os"

	"github.com/c9s/bbgo/pkg/types"
	"github.com/wcharczuk/go-chart/v2"
)

type Chart struct {
	*chart.Chart
}

func Create(candles []Candle, symbol string, interval types.Interval, options *CandleChartOptions) *Chart {
	if len(candles) == 0 {
		logger.Warn("no candles to plot, returning empty chart")
		return &Chart{Chart: &chart.Chart{}}
	}

	if options == nil {
		options = &CandleChartOptions{
			IncludeVolume: true,
			Width:         1200,
			Height:        600,
			RangePadding:  0.15,
		}
	}

	// Calculate min and max values for price and volume
	minY, maxY := FindPriceRange(candles)
	maxY = maxY * (1 + options.RangePadding)
	minY = minY * (1 - options.RangePadding)

	padDuration := interval.Duration()
	startTime := candles[0].Time
	endTime := candles[len(candles)-1].Time
	ch := &chart.Chart{
		Title: fmt.Sprintf("%s %s (%s ~ %s)",
			symbol,
			interval.String(),
			startTime.Format("01/02 15:04"),
			endTime.Format("01/02 15:04"),
		),
		Width:  options.Width,
		Height: options.Height,
		XAxis: chart.XAxis{
			ValueFormatter: chart.TimeValueFormatterWithFormat("01/02 15:04"),
			Range: &chart.ContinuousRange{
				Min: chart.TimeToFloat64(candles[0].Time.Add(-padDuration)),
				Max: chart.TimeToFloat64(candles[len(candles)-1].Time.Add(padDuration)),
			},
		},
		YAxis: chart.YAxis{
			Range: &chart.ContinuousRange{
				Min: minY,
				Max: maxY,
			},
		},
		Series: []chart.Series{
			CandlestickSeries{Candles: candles},
		},
	}

	if options != nil && options.IncludeVolume {
		minVolume, maxVolume := FindVolumeRange(candles)
		maxVolume = maxVolume * 3

		ch.YAxis.Range.SetMin(minY - (maxY-minY)*0.6)
		ch.Series = append(ch.Series, VolumeSeries{Candles: candles})
		ch.YAxisSecondary = chart.YAxis{
			Range: &chart.ContinuousRange{
				Min: minVolume,
				Max: maxVolume,
			},
		}
	}

	return &Chart{Chart: ch}
}

func Save(graph *Chart, filename string) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}

	defer func() {
		if cerr := f.Close(); cerr != nil {
			logger.Warnf("failed to close file: %v", cerr)
		}
	}()

	if err := graph.Render(chart.PNG, f); err != nil {
		return err
	}

	return nil
}

type CandleChartOptions struct {
	IncludeVolume bool    `json:"include_volume" yaml:"include_volume"`
	RangePadding  float64 `json:"range_padding" yaml:"range_padding"`
	Width         int     `json:"width" yaml:"width"`
	Height        int     `json:"height" yaml:"height"`

	FileName string `json:"filename" yaml:"filename"`
}

// ConvertKLinesToCandles converts a slice of KLine to a slice of Candle.
func ConvertKLinesToCandles(kLines []types.KLine) []Candle {
	var candles []Candle
	for _, k := range kLines {
		candles = append(candles, Candle{
			Time:   k.StartTime.Time().UTC(),
			Open:   k.Open.Float64(),
			High:   k.High.Float64(),
			Low:    k.Low.Float64(),
			Close:  k.Close.Float64(),
			Volume: k.Volume.Float64(),
		})
	}

	return candles
}

// FindPriceRange calculates the Y-axis range for the chart.
func FindPriceRange(candles []Candle) (float64, float64) {
	if len(candles) == 0 {
		return 0., 0.
	}

	minY, maxY := candles[0].Low, candles[0].High
	for _, c := range candles {
		minY = min(minY, c.Open, c.Close)
		maxY = max(maxY, c.Open, c.Close)
	}

	return minY, maxY
}

func FindVolumeRange(candles []Candle) (float64, float64) {
	if len(candles) == 0 {
		return 0., 0.
	}

	minVolume, maxVolume := candles[0].Volume, candles[0].Volume
	for _, c := range candles {
		minVolume = min(minVolume, c.Volume)
		maxVolume = max(maxVolume, c.Volume)
	}

	return minVolume, maxVolume
}
