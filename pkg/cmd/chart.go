package cmd

import (
	"context"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/c9s/bbgo/pkg/bbgo"
	bbgochart "github.com/c9s/bbgo/pkg/chart/v1"
	indicatorv2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/spf13/cobra"
	"github.com/wcharczuk/go-chart/v2/drawing"
)

func init() {
	ChartCommand.Flags().String("session", "", "the exchange session name for charting")
	ChartCommand.Flags().String("symbol", "", "the symbol to chart, e.g. BTCUSDT")
	ChartCommand.Flags().String("interval", "", "the interval for the chart, e.g. 1h, 15m")
	ChartCommand.Flags().String("since", "", "chart from time")
	ChartCommand.Flags().String("until", "", "chart until time")
	RootCmd.AddCommand(ChartCommand)
}

var ChartCommand = &cobra.Command{
	Use:   "chart [--session=[exchange_name]] [--symbol=[symbol]] [[--since=yyyy/mm/dd]] [[--until=yyyy/mm/dd]] ",
	Short: "create charts from config file",
	RunE:  chart,
}

func chart(cmd *cobra.Command, args []string) error {
	if userConfig.ChartConfig == nil {
		return fmt.Errorf("chart config is missing in the config file")
	}

	sessionName, err := cmd.Flags().GetString("session")
	if err != nil {
		return err
	}
	if sessionName == "" {
		return fmt.Errorf("--session is required")
	}

	symbol, err := cmd.Flags().GetString("symbol")
	if err != nil {
		return err
	}
	if symbol == "" {
		return fmt.Errorf("--symbol is required")
	}

	since, err := cmd.Flags().GetString("since")
	if err != nil {
		return err
	}
	until, err := cmd.Flags().GetString("until")
	if err != nil {
		return err
	}
	var startTime, endTime time.Time
	if until != "" {
		endTime, err = time.Parse(time.RFC3339, until)
		if err != nil {
			return fmt.Errorf("invalid until time %s: %w", until, err)
		}
	} else {
		endTime = time.Now()
	}
	if since != "" {
		startTime, err = time.Parse(time.RFC3339, since)
		if err != nil {
			return fmt.Errorf("invalid since time %s: %w", since, err)
		}
	} else {
		startTime = endTime.Add(-time.Hour * 24 * 30)
	}

	rawInterval, err := cmd.Flags().GetString("interval")
	if err != nil {
		return err
	}
	interval := types.Interval(rawInterval)

	environ := bbgo.NewEnvironment()
	environ.ConfigureExchangeSessions(userConfig)

	session, found := environ.Sessions()[sessionName]
	if !found {
		return fmt.Errorf("session %s not found in the config", sessionName)
	}
	ctx := context.Background()
	klines, err := session.Exchange.QueryKLines(
		ctx,
		symbol,
		types.Interval(interval.String()),
		types.KLineQueryOptions{
			StartTime: &startTime,
			EndTime:   &endTime,
		},
	)
	// sort klines by start time, old -> new
	sort.Slice(klines, func(i, j int) bool {
		return klines[i].StartTime.Before(klines[j].StartTime.Time())
	})

	if err != nil {
		return fmt.Errorf("query klines error: %w", err)
	}

	var errDraw error
	for _, chartConfig := range userConfig.ChartConfig {
		switch chartConfig.Kind {
		case "kline":
			errDraw = drawKLineChart(klines, &chartConfig)
		case "supertrend":
			errDraw = drawSuperTrendChart(klines, &chartConfig)
		case "atr":
			errDraw = drawAtrChar(klines, &chartConfig)
		case "ttmsqueeze":
			errDraw = drawTTMSqueezeChart(klines, &chartConfig)
		}
		if errDraw != nil {
			return errDraw
		}
	}
	return nil
}

func drawKLineChart(klines []types.KLine, config *bbgo.ChartConfig) error {
	panel := bbgochart.NewPanel(&config.Options)
	panel.AddKLines(klines)

	var fname string
	if config.Options.Title != "" {
		fname = config.Options.Title + ".png"
	} else {
		fname = fmt.Sprintf("%s_%s_%s.png",
			klines[0].Symbol,
			klines[0].Interval,
			time.Now().Format("2006-01-02_150405"),
		)
	}
	f, err := os.Create(fname)
	defer func() {
		if cerr := f.Close(); cerr != nil {
			fmt.Printf("failed to close file: %v\n", cerr)
		}
	}()
	if err != nil {
		return err
	}

	return panel.Write(f)
}

func drawSuperTrendChart(klines []types.KLine, config *bbgo.ChartConfig) error {
	klineStream := indicatorv2.KLineStream{}
	superTrendStream := indicatorv2.SuperTrend(
		&klineStream,
		config.Options.Window,
		config.Options.Multiplier,
	)
	klineStream.BackFill(klines)

	var samples []bbgochart.BandSample
	for _, e := range superTrendStream.Entities() {
		sample := bbgochart.BandSample{
			Time:  e.Time,
			Value: nil,
		}
		v := e.Value()
		if e.Direction == 1 {
			sample.LowerBound = &v
		} else {
			sample.UpperBound = &v
		}
		samples = append(samples, sample)
	}
	series := bbgochart.NewBandIndicatorSeries(
		fmt.Sprintf(
			"SuperTrend(w%d, m%.2f)",
			config.Options.Window, config.Options.Multiplier),
		samples,
		&config.Options,
	)

	panel := bbgochart.NewPanel(&config.Options)
	panel.AddKLines(klines)
	panel.AddIndicator(series)

	var fname string
	if config.Options.Title != "" {
		fname = config.Options.Title + ".png"
	} else {
		fname = fmt.Sprintf("%s_%s_%s.png",
			klines[0].Symbol,
			klines[0].Interval,
			time.Now().Format("2006-01-02_150405"),
		)
	}
	f, err := os.Create(fname)
	defer func() {
		if cerr := f.Close(); cerr != nil {
			fmt.Printf("failed to close file: %v\n", cerr)
		}
	}()
	if err != nil {
		return err
	}

	return panel.Write(f)
}

func drawAtrChar(klines []types.KLine, config *bbgo.ChartConfig) error {
	klineStream := indicatorv2.KLineStream{}
	atrStream := indicatorv2.ATR2(
		&klineStream, config.Options.Window,
	)
	klineStream.BackFill(klines)

	var points []bbgochart.PointSample
	for i, v := range atrStream.Slice {
		startTime := klines[i].StartTime
		points = append(points, bbgochart.PointSample{
			Time:  startTime.Time(),
			Value: &v,
		})
	}
	series := bbgochart.NewLineIndicatorSeries(
		fmt.Sprintf("ATR(w%d)", config.Options.Window),
		points,
		&config.Options,
	)

	panel := bbgochart.NewPanel(&config.Options)
	panel.AddIndicator(series)

	var fname string
	if config.Options.Title != "" {
		fname = config.Options.Title + ".png"
	} else {
		fname = fmt.Sprintf("ATR_%s_%s_w%d_%s.png",
			klines[0].Symbol,
			klines[0].Interval,
			config.Options.Window,
			time.Now().Format("2006-01-02_150405"),
		)
	}
	f, err := os.Create(fname)
	defer func() {
		if cerr := f.Close(); cerr != nil {
			fmt.Printf("failed to close file: %v\n", cerr)
		}
	}()
	if err != nil {
		return err
	}
	return panel.Write(f)
}

func drawTTMSqueezeChart(klines []types.KLine, config *bbgo.ChartConfig) error {
	klineStream := indicatorv2.KLineStream{}
	ttmStream := indicatorv2.NewTTMSqueezeStream(&klineStream, config.Options.Window)

	// Collect TTM Squeeze data via callback
	var squeezeData []indicatorv2.TTMSqueeze
	ttmStream.OnUpdate(func(squeeze indicatorv2.TTMSqueeze) {
		squeezeData = append(squeezeData, squeeze)
	})

	klineStream.BackFill(klines)

	if len(squeezeData) == 0 {
		return fmt.Errorf("no TTM Squeeze data generated")
	}

	// Find padding by matching first squeeze timestamp with klines
	firstSqueezeTime := squeezeData[0].Time.Time()
	var paddingTimestamps []types.Time
	for _, k := range klines {
		if !k.StartTime.Time().Before(firstSqueezeTime) {
			break
		}
		paddingTimestamps = append(paddingTimestamps, k.StartTime)
	}

	// Create histogram samples for momentum with padding
	histogramSamples := make([]bbgochart.ColumnSample, 0, len(klines))
	// Add padding samples (zero value, transparent)
	for _, t := range paddingTimestamps {
		histogramSamples = append(histogramSamples, bbgochart.ColumnSample{
			Time:  t.Time(),
			Value: 0,
		})
	}
	// Add actual squeeze data
	alpha := config.Options.ColumnAlpha
	if alpha == 0 {
		alpha = 100 // default alpha
	}
	for _, s := range squeezeData {
		var color drawing.Color
		switch s.MomentumDirection {
		case indicatorv2.MomentumDirectionBullish:
			color = drawing.Color{R: 0, G: 255, B: 255, A: alpha} // Aqua/Cyan
		case indicatorv2.MomentumDirectionBullishSlowing:
			color = drawing.Color{R: 0, G: 0, B: 255, A: alpha} // Blue
		case indicatorv2.MomentumDirectionBearish:
			color = drawing.Color{R: 255, G: 0, B: 0, A: alpha} // Red
		case indicatorv2.MomentumDirectionBearishSlowing:
			color = drawing.Color{R: 255, G: 255, B: 0, A: alpha} // Yellow
		default:
			color = drawing.Color{R: 128, G: 128, B: 128, A: alpha} // Gray for neutral/unknown
		}
		histogramSamples = append(histogramSamples, bbgochart.ColumnSample{
			Time:  s.Time.Time(),
			Value: s.Momentum,
			Color: color,
		})
	}
	histogramSeries := bbgochart.NewColumnIndicatorSeries(
		fmt.Sprintf("TTM Squeeze Momentum(w%d)", config.Options.Window),
		histogramSamples,
		&config.Options,
	)

	// Create dot samples for squeeze state with padding
	dotSamples := make([]bbgochart.DotSample, 0, len(klines))
	// Add padding samples (zero Y, transparent)
	for _, t := range paddingTimestamps {
		dotSamples = append(dotSamples, bbgochart.DotSample{
			Time: t.Time(),
			Y:    0,
		})
	}
	// Add actual squeeze data
	for _, s := range squeezeData {
		var color drawing.Color
		switch s.CompressionLevel {
		case indicatorv2.CompressionLevelNone:
			color = drawing.Color{R: 0, G: 255, B: 0, A: 255} // Green
		case indicatorv2.CompressionLevelLow:
			color = drawing.Color{R: 0, G: 0, B: 0, A: 255} // Black
		case indicatorv2.CompressionLevelMedium:
			color = drawing.Color{R: 255, G: 0, B: 0, A: 255} // Red
		case indicatorv2.CompressionLevelHigh:
			color = drawing.Color{R: 255, G: 165, B: 0, A: 255} // Orange
		default:
			color = drawing.Color{R: 60, G: 0, B: 128, A: 255} // Purple for unknown level
		}
		dotSamples = append(dotSamples, bbgochart.DotSample{
			Time:  s.Time.Time(),
			Y:     0,
			Color: color,
		})
	}
	dotSeries := bbgochart.NewDotIndicatorSeries(
		"Squeeze",
		dotSamples,
		&config.Options,
	)

	panel := bbgochart.NewPanel(&config.Options)
	panel.AddIndicator(histogramSeries)
	panel.AddIndicator(dotSeries)

	var fname string
	if config.Options.Title != "" {
		fname = config.Options.Title + ".png"
	} else {
		fname = fmt.Sprintf("TTMSqueeze_%s_%s_w%d_%s.png",
			klines[0].Symbol,
			klines[0].Interval,
			config.Options.Window,
			time.Now().Format("2006-01-02_150405"),
		)
	}
	f, err := os.Create(fname)
	defer func() {
		if cerr := f.Close(); cerr != nil {
			fmt.Printf("failed to close file: %v\n", cerr)
		}
	}()
	if err != nil {
		return err
	}
	return panel.Write(f)
}
