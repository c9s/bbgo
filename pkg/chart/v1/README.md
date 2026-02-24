# Chart API Design (v1)

This package provides a charting API for visualizing kline (candlestick) data with technical indicators.

## Overview

The chart API is designed to:
1. Render candlestick charts from kline data
2. Support overlaying multiple technical indicators
3. Provide a flexible annotation system for custom visualizations
4. Include volume information as a secondary series

## Core Components

### Chart Structure

The main `Chart` struct manages the overall charting functionality:

```go
type Chart struct {
    *chart.Chart              // Embedded go-chart.Chart
    klines     []types.KLine  // Source kline data
    indicators []Indicator    // Attached indicators
}
```

### Creating a Chart

```go
import bbgochart "github.com/c9s/bbgo/pkg/chart/v1"

// Create a new chart
graph := bbgochart.NewChart()

// Draw candles with options
graph.DrawCandles(klines, &bbgochart.CandleChartOptions{
    IncludeVolume: true,
    Width:         1200,
    Height:        600,
    RangePadding:  0.15,
})

// Save to file
bbgochart.Save(graph, "klines.png")
```

## Indicator Support

### Indicator Architecture

An indicator is a time series that annotates the kline chart. The system uses a flexible annotation provider pattern:

```go
type AnnotateFunc func(chart.Renderer, chart.Box, chart.Range, chart.Range, chart.Style)

type AnnotationProvider interface {
    GetAnnotation([]types.KLine, *AnnotationOptions) AnnotateFunc
}
```

The `AnnotationProvider` interface allows any indicator to define how it should be rendered on the chart, given the candle data and annotation options.

### How Indicators Work

1. **Data Flow**: Indicators receive candle data converted from klines
2. **Calculation**: Each indicator implements its own calculation logic
3. **Rendering**: The `GetAnnotation` method returns a function that draws the indicator on the chart
4. **Composition**: Multiple indicators can be added to a single chart

### Adding Indicators to Charts

```go
// Given a custom annotation provider constructor, `NewSMAAnnotationProvider`,
// We create an instance for a 20-period SMA
smaAnnotationProvider := NewSMAAnnotationProvider(20)

// Add an indicator with a custom annotation provider
graph.AddIndicator("SMA-20", smaAnnotationProvider, annotateOptions)

// The indicator will be rendered when DrawCandles is called
graph.DrawCandles(klines, options)
```

### Implementing Custom Indicators

To create a custom indicator, implement the `AnnotationProvider` interface:

```go
type MyIndicator struct {
    // Your indicator fields (e.g., window size, decay factor, etc.)
}

// GetAnnotation returns a function that renders the indicator
func (ind *MyIndicator) GetAnnotation(klines []types.KLine, options *AnnotationOptions) AnnotateFunc {
    return func(r chart.Renderer, b chart.Box, xRange, yRange chart.Range, style chart.Style) {
        // Calculate indicator values from klines
        ...
        
        // Draw the indicator (e.g., one kline at a time)
        for i := 0; i < len(klines)-1; i++ {
            // draw indicator value for klines[i]
        }
    }
}
```

## Integration with BBGO Indicators (v2)

The chart package is designed to work with BBGO's modern stream-based indicator library (`pkg/indicator/v2`). The v2 indicators use a **reactive stream architecture** where indicators are composed as data processing pipelines.

### Core V2 Indicator Pattern

V2 indicators follow a functional, compositional pattern:

```go
// SMAStream wraps a Float64Series and calculates SMA
type SMAStream struct {
    *types.Float64Series  // Embeds the result series
    window    int
    rawValues *types.Queue
}

// SMA creates a moving average stream from a float64 source
func SMA(source types.Float64Source, window int) *SMAStream {
    s := &SMAStream{
        Float64Series: types.NewFloat64Series(),
        window:        window,
        rawValues:     types.NewQueue(window),
    }
    // Bind to the source - process each value as it arrives
    s.Bind(source, s)
    return s
}

// Calculate processes each incoming value
func (s *SMAStream) Calculate(v float64) float64 {
    s.rawValues.Update(v)
    sma := s.rawValues.Mean(s.window)
    return sma
}
```

### Data Flow Pipeline

V2 indicators chain together to create processing pipelines:

```go
// Example: Bollinger Bands composed from SMA and StdDev
func BOLL(source types.Float64Source, window int, k float64) *BOLLStream {
    // Create component streams
    sma := SMA(source, window)
    stdDev := StdDev(source, window)
    
    s := &BOLLStream{
        Float64Series: types.NewFloat64Series(),
        UpBand:        types.NewFloat64Series(),
        DownBand:      types.NewFloat64Series(),
        SMA:           sma,
        StdDev:        stdDev,
    }
    
    // Calculations happen automatically via subscriptions
    s.Bind(source, s)
    return s
}
```

### Bridging V2 Indicators to Chart Annotations

To use a v2 indicator with the chart API, create an adapter that subscribes to the indicator stream:

```go
type SMAAnnotationProvider struct {
    window       int
    interval     types.Interval
}

func NewSMAAnnotationProvider(interval types.Interval, window int) *SMAAnnotationProvider {
    return &SMAAnnotationProvider{
        window:   window,
        interval: interval,
    }
}

func (p *SMAAnnotationProvider) GetAnnotation(klines []types.KLine, options *AnnotationOptions) AnnotateFunc {
    // Create the SMA stream
    klineStream := &indicatorv2.KLineStream{}
    closePriceStream := indicatorv2.ClosePrices(klineStream)
    sma := indicatorv2.SMA(closePriceStream, p.window)
    
    // Return rendering function that draws the SMA line
    return func(r chart.Renderer, b chart.Box, xRange, yRange chart.Range, style chart.Style) {
        // Populate the kline stream with historical data for backfill 
        for _, kline := range klines {
            klineStream.BackFill([]types.KLine{kline})
            if sma.Len() > 0 {
                // Draw the SMA value for the current kline
                ...
            }
        }
    }
}
```

### Key Advantages of V2

1. **Automatic Calculation**: Indicators update automatically when new data arrives
2. **Composability**: Complex indicators built from simpler ones
3. **Memory Efficiency**: Built-in truncation prevents unbounded growth
4. **Type Safety**: Strong typing for different data streams
5. **Historical Backfill**: Streams can replay historical data for indicators

## Helper Functions

The package provides utility functions for coordinate conversion:

```go
// Convert Y value (price) to canvas coordinates
func YValueToCanvas(r chart.Range, b chart.Box, v float64) int

// Convert X value (time) to canvas coordinates
func XValueToCanvas(r chart.Range, b chart.Box, v float64) int
```

## Example: Complete Chart with Indicators

```go
type SMAAnnotationProvider struct {
    window       int
    interval     types.Interval
}

func NewSMAAnnotationProvider(interval types.Interval, window int) *SMAAnnotationProvider {
    return &SMAAnnotationProvider{
        window:   window,
        interval: interval,
    }
}

func (p *SMAAnnotationProvider) GetAnnotation(klines []types.KLine, options *AnnotationOptions) AnnotateFunc {
    // Create the SMA stream
    klineStream := &indicatorv2.KLineStream{}
    closePriceStream := indicatorv2.ClosePrices(klineStream)
    sma := indicatorv2.SMA(closePriceStream, p.window)
    
    // Return rendering function that draws the SMA line
    return func(r chart.Renderer, b chart.Box, xRange, yRange chart.Range, style chart.Style) {
        // Populate the kline stream with historical data for backfill 
        for _, kline := range klines {
            klineStream.BackFill([]types.KLine{kline})
            if sma.Len() > 0 {
                // Draw the SMA value for the current kline
                ...
            }
        }
    }
}

// Create chart
graph := bbgochart.NewChart()

// Add indicators
sma20 := bbgochart.NewSMAAnnotationProvider(interval, 20)
sma50 := bbgochart.NewSMAAnnotationProvider(interval, 50)
graph.AddIndicator("SMA-20", sma20, v1.AnnotationOptions{})
graph.AddIndicator("SMA-50", sma50, v1.AnnotationOptions{})

// Draw chart with candles and indicators
graph.DrawCandles(klines, &v1.CandleChartOptions{
    IncludeVolume: true,
    Width:         1920,
    Height:        1080,
    RangePadding:  0.1,
})

// Save output
if err := v1.Save(graph, "sma_chart.png"); err != nil {
    ...
}
```

This example demonstrates:
1. **Custom Annotation Providers**: `SMAAnnotationProvider` implement the `AnnotationProvider` interface
2. **V2 Indicator Integration**: Uses `indicatorv2.SMA()` with stream-based architecture
3. **Backfilling**: Historical klines are fed into the indicator streams via `BackFill()`
4. **Rendering**: Each provider's `AnnotateFunc` draws lines using the chart renderer
5. **Multiple Indicators**: The chart overlays two indicators (SMA-20 and SMA-50)

## Design Philosophy

1. **Separation of Concerns**: Indicator calculation is separate from rendering
2. **Flexibility**: Any visualization can be implemented via `AnnotateFunc`
3. **Composability**: Multiple indicators can be overlaid on the same chart
4. **Stream-Based Integration**: Works seamlessly with BBGO's v2 reactive indicator streams
5. **Future-Proof**: `AnnotationOptions` allows for extensibility without breaking changes

# References
- https://github.com/wcharczuk/go-chart
- https://github.com/go-echarts/go-echarts