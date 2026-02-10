package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/c9s/bbgo/pkg/bbgo"
	bbgochart "github.com/c9s/bbgo/pkg/chart/v1"
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
	if err != nil {
		return fmt.Errorf("query klines error: %w", err)
	}

	graph := bbgochart.NewChart()
	graph.DrawCandles(klines, userConfig.ChartConfig)
	fileName := userConfig.ChartConfig.FileName
	if fileName == "" {
		fileName = "klines.png"
	}
	return bbgochart.Save(graph, fileName)
}
