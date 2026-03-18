package v1

import (
	"fmt"
	"io"
	"sort"
	"time"

	"github.com/c9s/bbgo/pkg/types"
	"github.com/wcharczuk/go-chart/v2"
)

type Panel struct {
	*chart.Chart

	Options *PanelOptions

	klines     []types.KLine
	indicators []IndicatorSeries
}

func NewPanel(options *PanelOptions) *Panel {
	if options == nil {
		options = &PanelOptions{
			IncludeVolume: true,
			RangePadding:  0.15,
			Width:         1200,
			Height:        600,
		}
	}

	return &Panel{
		Chart:   &chart.Chart{},
		Options: options,
	}
}

func (p *Panel) AddKLines(klines []types.KLine) {
	p.klines = append(p.klines, klines...)
	if len(p.klines) > len(klines) {
		// sort kline by start time
		sort.Slice(p.klines, func(i, j int) bool {
			return p.klines[i].StartTime.Before(p.klines[j].StartTime.Time())
		})
	}
}

func (p *Panel) draw() {
	var xAxis chart.XAxis
	var yAxis chart.YAxis
	candles := ConvertKLinesToCandles(p.klines)

	if len(candles) > 0 {
		// setup x and y axis based on klines
		symbol := p.klines[0].Symbol
		interval := p.klines[0].Interval

		// Calculate min and max values for price and volume
		minY, maxY := FindPriceRange(candles)
		maxY = maxY * (1 + p.Options.RangePadding)
		minY = minY * (1 - p.Options.RangePadding)

		minTime := candles[0].Time
		maxTime := candles[len(candles)-1].Time
		padDuration := interval.Duration()
		xAxis = chart.XAxis{
			ValueFormatter: chart.TimeValueFormatterWithFormat("01/02 15:04"),
			Range: &chart.ContinuousRange{
				Min: chart.TimeToFloat64(minTime.Add(-padDuration)),
				Max: chart.TimeToFloat64(maxTime.Add(padDuration)),
			},
		}
		yAxis = chart.YAxis{
			Range: &chart.ContinuousRange{
				Min: minY,
				Max: maxY,
			},
		}

		if p.Options.Title == "" {
			p.Title = fmt.Sprintf("%s %s (%s ~ %s)",
				symbol,
				interval.String(),
				minTime.Format("01/02 15:04"),
				maxTime.Format("01/02 15:04"),
			)
		} else {
			p.Title = p.Options.Title
		}
	} else {
		// find time range and value range from indicators
		var minTime, maxTime time.Time
		var minY, maxY float64

		for _, ind := range p.indicators {
			indMinTime, indMaxTime := ind.GetTimeRange()
			indMinValue, indMaxValue := ind.GetValueRange()
			if minTime.IsZero() || indMinTime.Before(minTime) {
				minTime = indMinTime
			}
			if maxTime.IsZero() || indMaxTime.After(maxTime) {
				maxTime = indMaxTime
			}
			if minY == 0.0 || indMinValue < minY {
				minY = indMinValue
			}
			if maxY == 0.0 || indMaxValue > maxY {
				maxY = indMaxValue
			}
		}

		p.Title = p.Options.Title
		xAxis = chart.XAxis{
			ValueFormatter: chart.TimeValueFormatterWithFormat("01/02 15:04"),
			Range: &chart.ContinuousRange{
				Min: chart.TimeToFloat64(minTime) - p.Options.XAxisPadding,
				Max: chart.TimeToFloat64(maxTime) + p.Options.XAxisPadding,
			},
		}
		yAxis = chart.YAxis{
			Range: &chart.ContinuousRange{
				Min: minY * (1 - p.Options.RangePadding),
				Max: maxY * (1 + p.Options.RangePadding),
			},
		}
	}

	p.Width = p.Options.Width
	p.Height = p.Options.Height
	p.XAxis = xAxis
	p.YAxis = yAxis

	// add series
	if len(p.klines) > 0 {
		p.Series = append(p.Series, &CandlestickSeries{Candles: candles})
		if p.Options != nil && p.Options.IncludeVolume {
			// draw volume
			minVolume, maxVolume := FindVolumeRange(candles)
			maxVolume = maxVolume * 3

			yMinAdj := p.YAxis.Range.GetMin() - p.YAxis.Range.GetDelta()*0.6
			yAxis.Range.SetMin(yMinAdj)
			p.YAxisSecondary = chart.YAxis{
				Range: &chart.ContinuousRange{
					Min: minVolume,
					Max: maxVolume,
				},
			}
			p.Series = append(p.Series, &VolumeSeries{Candles: candles})
		}
	}
	for _, ind := range p.indicators {
		p.Series = append(p.Series, ind)
	}
	if p.Options.Legend != nil {
		switch *p.Options.Legend {
		case LegendTop:
			p.Elements = append(p.Elements, chart.Legend(p.Chart))
		case LegendLeft:
			p.Elements = append(p.Elements, chart.LegendLeft(p.Chart))
		case LegendThin:
			p.Elements = append(p.Elements, chart.LegendThin(p.Chart))
		default:
			p.Elements = append(p.Elements, chart.Legend(p.Chart))
			logger.Warnf("unknown legend kind %s, use default legend top", *p.Options.Legend)
		}
	}
}

func (p *Panel) AddIndicator(indicator IndicatorSeries) {
	p.indicators = append(p.indicators, indicator)
}

func (p *Panel) Write(w io.Writer) error {
	p.draw()
	return p.Chart.Render(chart.PNG, w)
}

type PanelOptions struct {
	// general options
	Title        string      `json:"title,omitempty" yaml:"title,omitempty"`
	RangePadding float64     `json:"range_padding" yaml:"range_padding"`
	XAxisPadding float64     `json:"x_axis_padding" yaml:"x_axis_padding"`
	Width        int         `json:"width" yaml:"width"`
	Height       int         `json:"height" yaml:"height"`
	Legend       *LegendKind `json:"legend" yaml:"legend"`

	// kline options
	IncludeVolume bool `json:"include_volume" yaml:"include_volume"`

	// indicators options
	Window int `json:"window" yaml:"window"`

	// band indicators options
	UpperBoundColor string `json:"upper_bound_color" yaml:"upper_bound_color"`
	LowerBoundColor string `json:"lower_bound_color" yaml:"lower_bound_color"`
	ValueColor      string `json:"value_color" yaml:"value_color"`

	// column indicators options
	ColumnWidth float64 `json:"column_width" yaml:"column_width"`

	// dot indicators options
	DotRadius float64 `json:"dot_radius" yaml:"dot_radius"`

	// supertrend
	Multiplier float64 `json:"multiplier" yaml:"multiplier"`
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
