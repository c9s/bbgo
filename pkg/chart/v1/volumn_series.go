package v1

import "github.com/wcharczuk/go-chart/v2"

type VolumeSeries struct {
	Candles []Candle
}

func (vs VolumeSeries) Len() int {
	return len(vs.Candles)
}

func (vs VolumeSeries) GetValue(index int) float64 {
	return vs.Candles[index].Volume
}

func (vs VolumeSeries) GetXAxisValue(index int) float64 {
	return chart.TimeToFloat64(vs.Candles[index].Time)
}

func (vs VolumeSeries) GetFirstValues() (x, y float64) {
	if len(vs.Candles) == 0 {
		return 0, 0
	}
	x = chart.TimeToFloat64(vs.Candles[0].Time)
	y = vs.Candles[0].Volume
	return
}

func (vs VolumeSeries) GetLastValues() (x, y float64) {
	if len(vs.Candles) == 0 {
		return 0, 0
	}
	x = chart.TimeToFloat64(vs.Candles[len(vs.Candles)-1].Time)
	y = vs.Candles[len(vs.Candles)-1].Volume
	return
}

func (vs VolumeSeries) GetValueFormatters() (x, y chart.ValueFormatter) {
	x = chart.TimeValueFormatter
	y = chart.FloatValueFormatter
	return
}

func (vs VolumeSeries) GetName() string {
	return "Volume"
}

func (vs VolumeSeries) GetStyle() chart.Style {
	return chart.Style{
		StrokeWidth: 1.0,
		FillColor:   chart.ColorBlue.WithAlpha(50),
	}
}

func (vs VolumeSeries) GetYAxis() chart.YAxisType {
	return chart.YAxisSecondary
}

func (vs VolumeSeries) Render(r chart.Renderer, canvasBox chart.Box, xrange, yrange chart.Range, style chart.Style) {
	if len(vs.Candles) == 0 {
		return
	}

	barWidth := float64(canvasBox.Width()) / float64(len(vs.Candles))
	for _, candle := range vs.Candles {
		x := chart.TimeToFloat64(candle.Time)
		xp := XValueToCanvas(xrange, canvasBox, x)
		yVolume := YValueToCanvas(yrange, canvasBox, candle.Volume)
		left := int(float64(xp) - barWidth/2)
		right := int(float64(xp) + barWidth/2)
		if right == left {
			right = left + 1
		}

		if style.FillColor.IsZero() {
			style.FillColor = chart.ColorBlue.WithAlpha(50)
		}

		r.SetFillColor(style.FillColor)
		r.MoveTo((left), (canvasBox.Bottom))
		r.LineTo((left), (yVolume))
		r.LineTo((right), (yVolume))
		r.LineTo((right), (canvasBox.Bottom))
		r.LineTo((left), (canvasBox.Bottom))
		r.Fill()
	}
}

// XValues returns all X axis values for the series.
func (vs VolumeSeries) XValues() []float64 {
	values := make([]float64, len(vs.Candles))
	for i, c := range vs.Candles {
		values[i] = chart.TimeToFloat64(c.Time)
	}
	return values
}

func (vs VolumeSeries) Validate() error {
	if len(vs.Candles) == 0 {
		return nil
	}
	return nil
}
