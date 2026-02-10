package v1

import (
	"fmt"
	"os"
	"sort"

	"github.com/c9s/bbgo/pkg/types"
	"github.com/wcharczuk/go-chart/v2"
)

type Chart struct {
	*chart.Chart

	klines []types.KLine
}

func NewChart() *Chart {
	return &Chart{
		Chart: &chart.Chart{},
	}
}

func (c *Chart) DrawCandles(klines []types.KLine, options *CandleChartOptions) {
	if len(c.klines) > 0 {
		c.klines = append(c.klines, klines...)
		// sort kline by start time
		sort.Slice(c.klines, func(i, j int) bool {
			return c.klines[i].StartTime.Before(c.klines[j].StartTime.Time())
		})
	}

	candles := ConvertKLinesToCandles(c.klines)
	if len(candles) == 0 {
		logger.Warn("no candles to plot")
		return
	}

	if options == nil {
		options = &CandleChartOptions{
			IncludeVolume: true,
			Width:         1200,
			Height:        600,
			RangePadding:  0.15,
		}
	}

	symbol := c.klines[0].Symbol
	interval := c.klines[0].Interval

	// Calculate min and max values for price and volume
	minY, maxY := FindPriceRange(candles)
	maxY = maxY * (1 + options.RangePadding)
	minY = minY * (1 - options.RangePadding)

	padDuration := interval.Duration()
	startTime := candles[0].Time
	endTime := candles[len(candles)-1].Time
	c.Title = fmt.Sprintf("%s %s (%s ~ %s)",
		symbol,
		interval.String(),
		startTime.Format("01/02 15:04"),
		endTime.Format("01/02 15:04"),
	)
	c.Width = options.Width
	c.Height = options.Height
	c.XAxis = chart.XAxis{
		ValueFormatter: chart.TimeValueFormatterWithFormat("01/02 15:04"),
		Range: &chart.ContinuousRange{
			Min: chart.TimeToFloat64(candles[0].Time.Add(-padDuration)),
			Max: chart.TimeToFloat64(candles[len(candles)-1].Time.Add(padDuration)),
		},
	}
	c.YAxis = chart.YAxis{
		Range: &chart.ContinuousRange{
			Min: minY,
			Max: maxY,
		},
	}
	series := []chart.Series{
		CandlestickSeries{
			Candles: candles,
		},
	}
	for _, s := range c.Series {
		_, ok := s.(CandlestickSeries)
		if ok {
			continue
		}
		series = append(series, s)
	}
	c.Series = series

	if options != nil && options.IncludeVolume {
		minVolume, maxVolume := FindVolumeRange(candles)
		maxVolume = maxVolume * 3

		c.YAxis.Range.SetMin(minY - (maxY-minY)*0.6)
		c.Series = append(c.Series, VolumeSeries{Candles: candles})
		c.YAxisSecondary = chart.YAxis{
			Range: &chart.ContinuousRange{
				Min: minVolume,
				Max: maxVolume,
			},
		}
	}
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
