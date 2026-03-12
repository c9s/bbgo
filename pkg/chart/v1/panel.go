package v1

import (
	"fmt"
	"io"
	"sort"

	"github.com/c9s/bbgo/pkg/types"
	"github.com/wcharczuk/go-chart/v2"
)

type Panel struct {
	*chart.Chart

	Name    string
	Options *PanelOptions

	klines     []types.KLine
	indicators []IndicatorSeries
}

func NewPanel(name string, options *PanelOptions) *Panel {
	if options == nil {
		options = &PanelOptions{
			IncludeVolume: true,
			RangePadding:  0.15,
			Width:         1200,
			Height:        600,
		}
	}

	return &Panel{
		Chart: &chart.Chart{},

		Name:    name,
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
	candles := ConvertKLinesToCandles(p.klines)
	if len(candles) == 0 {
		logger.Warn("no candles to plot")
		return
	}

	symbol := p.klines[0].Symbol
	interval := p.klines[0].Interval

	// Calculate min and max values for price and volume
	minY, maxY := FindPriceRange(candles)
	maxY = maxY * (1 + p.Options.RangePadding)
	minY = minY * (1 - p.Options.RangePadding)

	padDuration := interval.Duration()
	startTime := candles[0].Time
	endTime := candles[len(candles)-1].Time
	p.Title = fmt.Sprintf("%s %s (%s ~ %s)",
		symbol,
		interval.String(),
		startTime.Format("01/02 15:04"),
		endTime.Format("01/02 15:04"),
	)
	p.Width = p.Options.Width
	p.Height = p.Options.Height
	p.XAxis = chart.XAxis{
		ValueFormatter: chart.TimeValueFormatterWithFormat("01/02 15:04"),
		Range: &chart.ContinuousRange{
			Min: chart.TimeToFloat64(candles[0].Time.Add(-padDuration)),
			Max: chart.TimeToFloat64(candles[len(candles)-1].Time.Add(padDuration)),
		},
	}
	p.YAxis = chart.YAxis{
		Range: &chart.ContinuousRange{
			Min: minY,
			Max: maxY,
		},
	}
	p.Series = []chart.Series{
		CandlestickSeries{
			Candles: candles,
		},
	}
	for _, ind := range p.indicators {
		p.Series = append(p.Series, ind)
		legendKind := ind.GetLegend()
		if legendKind != nil {
			switch *legendKind {
			case LegendTop:
				p.Elements = append(p.Elements, chart.Legend(p.Chart))
			case LegendLeft:
				p.Elements = append(p.Elements, chart.LegendLeft(p.Chart))
			case LegendThin:
				p.Elements = append(p.Elements, chart.LegendThin(p.Chart))
			default:
				p.Elements = append(p.Elements, chart.Legend(p.Chart))
				logger.Warnf("unknown legend kind %s, use default legend top", *legendKind)
			}
		}
	}

	if p.Options != nil && p.Options.IncludeVolume {
		minVolume, maxVolume := FindVolumeRange(candles)
		maxVolume = maxVolume * 3

		p.YAxis.Range.SetMin(minY - (maxY-minY)*0.6)
		p.Series = append(p.Series, VolumeSeries{Candles: candles})
		p.YAxisSecondary = chart.YAxis{
			Range: &chart.ContinuousRange{
				Min: minVolume,
				Max: maxVolume,
			},
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
	IncludeVolume bool    `json:"include_volume" yaml:"include_volume"`
	RangePadding  float64 `json:"range_padding" yaml:"range_padding"`
	Width         int     `json:"width" yaml:"width"`
	Height        int     `json:"height" yaml:"height"`

	// band indicators options
	UpperBoundColor string `json:"upper_bound_color" yaml:"upper_bound_color"`
	LowerBoundColor string `json:"lower_bound_color" yaml:"lower_bound_color"`
	ValueColor      string `json:"value_color" yaml:"value_color"`
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
