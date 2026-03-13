# Chart API (v1)

This package provides a charting API for visualizing kline (candlestick) data with technical indicators, built on top of [go-chart](https://github.com/wcharczuk/go-chart).

## Overview

The chart API is designed to:
1. Render candlestick charts from kline data
2. Overlay multiple technical indicators as separate `chart.Series`
3. Include volume as a secondary series
4. Write charts as PNG to any `io.Writer`

## Core Components

### Panel

`Panel` is the top-level struct that manages the chart. It embeds `*chart.Chart` and holds klines plus any indicator series.

```go
type Panel struct {
    *chart.Chart
    Name    string
    Options *PanelOptions
    // ...
}
```

### PanelOptions

```go
type PanelOptions struct {
    IncludeVolume   bool    `json:"include_volume" yaml:"include_volume"`
    RangePadding    float64 `json:"range_padding" yaml:"range_padding"`
    Width           int     `json:"width" yaml:"width"`
    Height          int     `json:"height" yaml:"height"`

    // Colors for indicator series (CSS hex or named colors)
    UpperBoundColor string `json:"upper_bound_color" yaml:"upper_bound_color"`
    LowerBoundColor string `json:"lower_bound_color" yaml:"lower_bound_color"`
    ValueColor      string `json:"value_color" yaml:"value_color"`
}
```

### Creating and Rendering a Chart

```go
import (
    "log"
    "os"

    bbgochart "github.com/c9s/bbgo/pkg/chart/v1"
)

panel := bbgochart.NewPanel("BTCUSDT", &bbgochart.PanelOptions{
    IncludeVolume: true,
    RangePadding:  0.15,
    Width:         1200,
    Height:        600,
})

panel.AddKLines(klines)

f, err := os.Create("klines.png")
if err != nil {
    log.Fatal(err)
}
defer f.Close()

if err := panel.Write(f); err != nil {
    log.Fatal(err)
}
```

`Write` triggers the internal layout pass, which builds the series list (always `CandlestickSeries`, then any registered indicator series, and optionally `VolumeSeries` when `IncludeVolume` is true), then renders the chart as PNG to the given `io.Writer`.

## Indicator Series

Indicators are added to the panel via `AddIndicator`, which accepts any value that satisfies `chart.Series`.

```go
panel.AddIndicator(series)
```

Two built-in series types are provided.

### LineIndicatorSeries

Renders a single line from a sequence of `(time, value)` points. Nil values break the line.

```go
type PointSample struct {
    Time  time.Time
    Value *float64  // nil = gap in the line
}
```

```go
line := bbgochart.NewLineIndicatorSeries("SMA-20", nil, &bbgochart.PanelOptions{
    ValueColor: "#0074D9",
})

v := 42000.0
line.AddPoints(bbgochart.PointSample{Time: t, Value: &v})

panel.AddIndicator(line)
```

### BandIndicatorSeries

Renders up to three lines — upper bound, lower bound, and a middle value — from a sequence of `BandSample` points. Each field is nullable; nil fields are skipped for that line only.

```go
type BandSample struct {
    Time                          time.Time
    UpperBound, LowerBound, Value *float64
}
```

```go
band := bbgochart.NewBandIndicatorSeries("BOLL", nil, &bbgochart.PanelOptions{
    UpperBoundColor: "#2ECC40",  // green upper
    LowerBoundColor: "#FF4136",  // red lower
    ValueColor:      "#AAAAAA",  // gray midline
})

upper, mid, lower := 43000.0, 42000.0, 41000.0
band.AddSamples(bbgochart.BandSample{
    Time:       t,
    UpperBound: &upper,
    Value:      &mid,
    LowerBound: &lower,
})

panel.AddIndicator(band)
```

## Example: Candlestick Chart with SMA and Bollinger Bands

```go
package main

import (
    "log"
    "os"
    "time"

    bbgochart "github.com/c9s/bbgo/pkg/chart/v1"
    indicatorv2 "github.com/c9s/bbgo/pkg/indicator/v2"
    "github.com/c9s/bbgo/pkg/types"
)

func buildChart(klines []types.KLine) error {
    panel := bbgochart.NewPanel("BTCUSDT 1h", &bbgochart.PanelOptions{
        IncludeVolume: true,
        RangePadding:  0.15,
        Width:         1920,
        Height:        1080,
    })
    panel.AddKLines(klines)

    // --- SMA-20 via LineIndicatorSeries ---
    smaSeries := bbgochart.NewLineIndicatorSeries("SMA-20", nil, &bbgochart.PanelOptions{
        ValueColor: "#0074D9",
    })

    klineStream := &indicatorv2.KLineStream{}
    closeStream := indicatorv2.ClosePrices(klineStream)
    sma := indicatorv2.SMA(closeStream, 20)
    for _, k := range klines {
        klineStream.BackFill([]types.KLine{k})
        if sma.Len() > 0 {
            v := sma.Last(0)
            smaSeries.AddPoints(bbgochart.PointSample{
                Time:  k.StartTime.Time(),
                Value: &v,
            })
        }
    }
    panel.AddIndicator(smaSeries)

    // --- Bollinger Bands via BandIndicatorSeries ---
    bollSeries := bbgochart.NewBandIndicatorSeries("BOLL-20", nil, &bbgochart.PanelOptions{
        UpperBoundColor: "#2ECC40",
        LowerBoundColor: "#FF4136",
        ValueColor:      "#AAAAAA",
    })

    klineStream2 := &indicatorv2.KLineStream{}
    boll := indicatorv2.BOLL(indicatorv2.ClosePrices(klineStream2), 20, 2.0)
    for _, k := range klines {
        klineStream2.BackFill([]types.KLine{k})
        if boll.UpBand.Len() > 0 {
            up := boll.UpBand.Last(0)
            mid := boll.Last(0)
            dn := boll.DownBand.Last(0)
            bollSeries.AddSamples(bbgochart.BandSample{
                Time:       k.StartTime.Time(),
                UpperBound: &up,
                Value:      &mid,
                LowerBound: &dn,
            })
        }
    }
    panel.AddIndicator(bollSeries)

    f, err := os.Create("chart.png")
    if err != nil {
        return err
    }
    defer f.Close()
    return panel.Write(f)
}
```

## Helper Functions

```go
// Convert a price value to a canvas Y coordinate
func YValueToCanvas(r chart.Range, b chart.Box, v float64) int

// Convert a time float64 to a canvas X coordinate
func XValueToCanvas(r chart.Range, b chart.Box, v float64) int

// Convert []types.KLine to []Candle
func ConvertKLinesToCandles(kLines []types.KLine) []Candle

// Return the min/max price across a slice of candles
func FindPriceRange(candles []Candle) (min, max float64)

// Return the min/max volume across a slice of candles
func FindVolumeRange(candles []Candle) (min, max float64)
```

## Design Philosophy

1. **`IndicatorSeries` as the integration point**: `AddIndicator` accepts the `IndicatorSeries` interface, which extends go-chart's `chart.Series` with `GetTimeRange() (time.Time, time.Time)` and `GetValueRange() (float64, float64)`. These extra methods let the panel compute axes when rendering indicator-only charts (no klines). The two built-in types — `LineIndicatorSeries` and `BandIndicatorSeries` — satisfy this interface, and any external type can too.
2. **Data accumulation before layout**: `AddKLines`, `AddPoints`, and `AddSamples` collect raw data upfront. `Write` triggers an internal `draw()` pass that assembles the full series list, axes, and ranges in a single shot. When klines are present, axes derive from their price/time range; when the panel contains only indicators, axes derive from `GetTimeRange`/`GetValueRange` across all registered indicator series.
3. **Nullable values for natural gaps**: `PointSample.Value` and the three fields of `BandSample` are `*float64`. A nil value breaks the line at that point rather than zero-filling or interpolating, which correctly handles indicator warm-up periods and missing data.
4. **Shared options struct**: `PanelOptions` carries panel-level settings (dimensions, padding, volume toggle, title, x-axis padding, legend kind) and indicator color configuration. Passing the same struct to both `NewPanel` and the indicator series constructors avoids a proliferation of separate config types.
5. **Composability over inheritance**: Multiple indicators of any `IndicatorSeries`-compatible type can be overlaid on a single panel. The panel stores `[]IndicatorSeries` and delegates rendering entirely to each series' own `Render` method.

## References

- https://github.com/wcharczuk/go-chart
- https://github.com/go-echarts/go-echarts
