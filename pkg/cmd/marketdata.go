package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/c9s/bbgo/pkg/marketdata"
	"github.com/c9s/bbgo/pkg/marketdata/sources/binancecsv"
	"github.com/c9s/bbgo/pkg/types"
)

// marketDataCmd groups the commands that exercise the market data ingestion
// layer on its own, without the backtest engine.
//
// The layer is verifiable this way: download the archives a range needs, then
// dump the merged stream and check it is ordered and complete.
var marketDataCmd = &cobra.Command{
	Use:     "marketdata",
	Short:   "download and inspect historical market data",
	Aliases: []string{"md"},
}

var marketDataDownloadCmd = &cobra.Command{
	Use:   "download",
	Short: "download historical market data archives into the local cache",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		src, req, err := marketDataSourceFromFlags(cmd)
		if err != nil {
			return err
		}

		dryRun, err := cmd.Flags().GetBool("dry-run")
		if err != nil {
			return err
		}

		if dryRun {
			refs, err := src.Files(req)
			if err != nil {
				return err
			}
			for _, ref := range refs {
				fmt.Println(ref.URL())
			}
			log.Infof("%d archives would be downloaded", len(refs))
			return nil
		}

		paths, err := src.FetchAll(ctx, req)
		if err != nil {
			return err
		}

		var total int64
		for _, path := range paths {
			if st, err := os.Stat(path); err == nil {
				total += st.Size()
			}
		}

		log.Infof("cached %d archives, %.1f MiB", len(paths), float64(total)/(1<<20))
		return nil
	},
}

var marketDataDumpCmd = &cobra.Command{
	Use:   "dump",
	Short: "decode and print the merged market data stream",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		src, req, err := marketDataSourceFromFlags(cmd)
		if err != nil {
			return err
		}

		limit, err := cmd.Flags().GetInt("limit")
		if err != nil {
			return err
		}
		countOnly, err := cmd.Flags().GetBool("count-only")
		if err != nil {
			return err
		}

		merged, err := marketdata.MergeSources(ctx, []marketdata.Source{src}, req)
		if err != nil {
			return err
		}
		defer merged.Close()

		var (
			count  int64
			byType = map[marketdata.EventType]int64{}
			prev   marketdata.OrderKey
		)

		for merged.Next() {
			ev := merged.Event()

			// The stream is asserted here rather than only in tests, because
			// this command is how a new dataset or provider gets validated.
			if count > 0 && ev.Key.Compare(prev) < 0 {
				return fmt.Errorf("stream is out of order at event %d: %+v after %+v",
					count, ev.Key, prev)
			}
			prev = ev.Key

			count++
			byType[ev.Type]++

			if !countOnly && (limit <= 0 || count <= int64(limit)) {
				fmt.Println(formatEvent(ev))
			}
		}

		if err := merged.Err(); err != nil {
			return err
		}

		log.Infof("%d events, in order", count)
		for _, s := range merged.Stats() {
			log.Infof("  source %s: %d events, %s .. %s",
				s.Name, s.Count,
				s.First.Format(time.RFC3339Nano), s.Last.Format(time.RFC3339Nano))
		}
		for evType, n := range byType {
			log.Infof("  %s: %d", evType, n)
		}

		return nil
	},
}

// formatEvent renders one event as a single line, showing the fields that
// matter for eyeballing correctness.
func formatEvent(ev *marketdata.Event) string {
	head := fmt.Sprintf("%s %-12s %-10s",
		ev.Time().Format("2006-01-02T15:04:05.000000000Z"), ev.Type, ev.Symbol)

	switch ev.Type {
	case marketdata.EventTypeTrade:
		return fmt.Sprintf("%s %-4s %s @ %s", head,
			ev.Trade.Side, ev.Trade.Quantity.String(), ev.Trade.Price.String())

	case marketdata.EventTypeKLine:
		return fmt.Sprintf("%s %-4s O %s H %s L %s C %s V %s", head,
			ev.KLine.Interval, ev.KLine.Open.String(), ev.KLine.High.String(),
			ev.KLine.Low.String(), ev.KLine.Close.String(), ev.KLine.Volume.String())

	case marketdata.EventTypeBookSnapshot, marketdata.EventTypeBookUpdate:
		return fmt.Sprintf("%s bids %d asks %d seq %d", head,
			len(ev.Book.Bids), len(ev.Book.Asks), ev.Key.Seq)

	case marketdata.EventTypeBookTicker:
		return fmt.Sprintf("%s %s x %s / %s x %s", head,
			ev.BookTicker.Buy.String(), ev.BookTicker.BuySize.String(),
			ev.BookTicker.Sell.String(), ev.BookTicker.SellSize.String())

	case marketdata.EventTypeDepthBand:
		return fmt.Sprintf("%s %s%% depth %s notional %s", head,
			ev.DepthBand.Percentage.String(), ev.DepthBand.Depth.String(),
			ev.DepthBand.Notional.String())

	case marketdata.EventTypeMetrics:
		return fmt.Sprintf("%s %v", head, ev.Metrics.Values)

	default:
		return head
	}
}

// marketDataSourceFromFlags builds a binancecsv source and a request from the
// command line. Configuration-file driven sources arrive with the registry.
func marketDataSourceFromFlags(cmd *cobra.Command) (*binancecsv.Source, marketdata.Request, error) {
	var empty marketdata.Request

	marketName, err := cmd.Flags().GetString("market")
	if err != nil {
		return nil, empty, err
	}
	market, err := binancecsv.ParseMarket(marketName)
	if err != nil {
		return nil, empty, err
	}

	datasets, err := cmd.Flags().GetStringSlice("dataset")
	if err != nil {
		return nil, empty, err
	}
	symbols, err := cmd.Flags().GetStringSlice("symbol")
	if err != nil {
		return nil, empty, err
	}
	intervalNames, err := cmd.Flags().GetStringSlice("interval")
	if err != nil {
		return nil, empty, err
	}
	periodName, err := cmd.Flags().GetString("period")
	if err != nil {
		return nil, empty, err
	}
	cacheDir, err := cmd.Flags().GetString("cache-dir")
	if err != nil {
		return nil, empty, err
	}
	allowMissing, err := cmd.Flags().GetBool("allow-missing")
	if err != nil {
		return nil, empty, err
	}

	since, until, err := marketDataRange(cmd)
	if err != nil {
		return nil, empty, err
	}

	if cacheDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, empty, err
		}
		cacheDir = filepath.Join(home, ".bbgo", "marketdata")
	}

	var intervals []types.Interval
	for _, name := range intervalNames {
		if name = strings.TrimSpace(name); name != "" {
			intervals = append(intervals, types.Interval(name))
		}
	}

	src, err := binancecsv.New(binancecsv.Config{
		Market:           market,
		Period:           binancecsv.Period(periodName),
		Datasets:         datasets,
		Symbols:          symbols,
		Intervals:        intervals,
		CacheDir:         cacheDir,
		AllowMissingDays: allowMissing,
	})
	if err != nil {
		return nil, empty, err
	}

	req := marketdata.Request{
		Since:         since,
		Until:         until,
		Subscriptions: marketDataSubscriptions(src, symbols, intervals),
	}

	return src, req, nil
}

// marketDataSubscriptions builds one subscription per symbol for every channel
// the configured datasets serve, so the CLI needs no separate channel flag.
func marketDataSubscriptions(
	src *binancecsv.Source, symbols []string, intervals []types.Interval,
) []types.Subscription {
	channels := src.Capabilities().Channels

	var subs []types.Subscription
	for _, symbol := range symbols {
		for _, channel := range channels {
			if channel == types.KLineChannel && len(intervals) > 0 {
				for _, interval := range intervals {
					subs = append(subs, types.Subscription{
						Symbol:  symbol,
						Channel: channel,
						Options: types.SubscribeOptions{Interval: interval},
					})
				}
				continue
			}
			subs = append(subs, types.Subscription{Symbol: symbol, Channel: channel})
		}
	}

	return subs
}

func marketDataRange(cmd *cobra.Command) (since, until time.Time, err error) {
	sinceStr, err := cmd.Flags().GetString("since")
	if err != nil {
		return since, until, err
	}
	untilStr, err := cmd.Flags().GetString("until")
	if err != nil {
		return since, until, err
	}

	if since, err = parseMarketDataTime(sinceStr); err != nil {
		return since, until, fmt.Errorf("bad --since: %w", err)
	}

	if untilStr == "" {
		until = time.Now().UTC().Truncate(24 * time.Hour)
	} else if until, err = parseMarketDataTime(untilStr); err != nil {
		return since, until, fmt.Errorf("bad --until: %w", err)
	}

	return since, until, nil
}

// parseMarketDataTime accepts a plain date or an RFC3339 timestamp. Archives
// are published per UTC day, so a bare date is interpreted as UTC midnight.
func parseMarketDataTime(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, fmt.Errorf("a time is required")
	}
	if t, err := time.ParseInLocation(time.DateOnly, s, time.UTC); err == nil {
		return t, nil
	}
	return time.Parse(time.RFC3339, s)
}

func init() {
	for _, cmd := range []*cobra.Command{marketDataDownloadCmd, marketDataDumpCmd} {
		cmd.Flags().String("market", "um", "archive tree: spot, um (USDⓈ-M) or cm (COIN-M)")
		cmd.Flags().String("period", "daily", "archive period: daily or monthly")
		cmd.Flags().StringSlice("dataset", []string{"aggTrades"},
			fmt.Sprintf("datasets to read, one or more of %v", binancecsv.DatasetNames()))
		cmd.Flags().StringSlice("symbol", nil, "symbols, e.g. BTCUSDT")
		cmd.Flags().StringSlice("interval", nil, "kline intervals, e.g. 1m,1h")
		cmd.Flags().String("since", "", "start of the range, YYYY-MM-DD or RFC3339 (inclusive)")
		cmd.Flags().String("until", "", "end of the range, YYYY-MM-DD or RFC3339 (exclusive)")
		cmd.Flags().String("cache-dir", "", "archive cache directory (default ~/.bbgo/marketdata)")
		cmd.Flags().Bool("allow-missing", false,
			"treat an archive the publisher does not have as a warning instead of an error")

		if err := cmd.MarkFlagRequired("symbol"); err != nil {
			panic(err)
		}
		if err := cmd.MarkFlagRequired("since"); err != nil {
			panic(err)
		}
	}

	marketDataDownloadCmd.Flags().Bool("dry-run", false, "print the archive URLs without downloading")
	marketDataDumpCmd.Flags().Int("limit", 20, "print at most this many events, 0 for all")
	marketDataDumpCmd.Flags().Bool("count-only", false, "only count events, do not print them")

	marketDataCmd.AddCommand(marketDataDownloadCmd)
	marketDataCmd.AddCommand(marketDataDumpCmd)
	RootCmd.AddCommand(marketDataCmd)
}
