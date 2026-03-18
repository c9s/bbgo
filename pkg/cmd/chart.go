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
			errDraw = drawAtrChart(klines, &chartConfig)
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

func drawAtrChart(klines []types.KLine, config *bbgo.ChartConfig) error {
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
