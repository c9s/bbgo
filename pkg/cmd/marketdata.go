package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/marketdata"
	"github.com/c9s/bbgo/pkg/marketdata/registry"
	"github.com/c9s/bbgo/pkg/marketdata/sources/binancecsv"
	"github.com/c9s/bbgo/pkg/marketdata/sources/replay"
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

		sources, req, err := marketDataDumpSources(cmd)
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
		checkBook, err := cmd.Flags().GetBool("check-book")
		if err != nil {
			return err
		}

		merged, err := marketdata.MergeSources(ctx, sources, req)
		if err != nil {
			return err
		}
		defer merged.Close()

		var (
			count  int64
			byType = map[marketdata.EventType]int64{}
			prev   marketdata.OrderKey
			books  = map[string]*marketdata.BookState{}
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

			if checkBook {
				book, ok := books[ev.Symbol]
				if !ok {
					book = marketdata.NewBookState(ev.Symbol, ev.Exchange)
					book.Mode = marketdata.SequenceContiguous
					book.CheckCrossed = true
					books[ev.Symbol] = book
				}
				if err := book.Apply(ev); err != nil {
					return fmt.Errorf("book check failed at event %d (%s): %w",
						count, ev.Time().Format(time.RFC3339Nano), err)
				}
			}

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

		for symbol, book := range books {
			bid, ask, ok := book.Book.BestBidAndAsk()
			if !ok {
				log.Warnf("  %s book is empty", symbol)
				continue
			}
			log.Infof("  %s book replayed to sequence %d, best %s / %s",
				symbol, book.LastSequence(), bid.Price.String(), ask.Price.String())

			if n := book.Unverified(); n > 0 {
				// Do not claim the book is intact when it cannot be proven:
				// without the venue's previous-update id, a lost update is
				// indistinguishable from a batched one.
				log.Warnf("  %s: %d updates could not be verified as contiguous, "+
					"because the source did not provide a previous-update id", symbol, n)
			} else {
				log.Infof("  %s: sequence verified contiguous", symbol)
			}
		}

		return nil
	},
}

var marketDataRecordCmd = &cobra.Command{
	Use:   "record",
	Short: "record a live market data stream for later replay",
	Long: `Record a live market data stream to a replayable file.

This exists because the public archives cannot supply L2: data.binance.vision
publishes no depth diffs, and the vendor APIs that do require a paid key. A
recording is therefore the only way to get real order book snapshots and updates
into a backtest without buying data.

Recordings rotate hourly and replay through the same marketdata.Source interface
as any other provider, so they merge with archive data in time order.`,
	PreRunE: cobraInitRequired([]string{"session", "symbol"}),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		environ := bbgo.NewEnvironment()
		if err := environ.ConfigureExchangeSessions(userConfig); err != nil {
			return err
		}

		sessionName, err := cmd.Flags().GetString("session")
		if err != nil {
			return err
		}

		session, ok := environ.Session(sessionName)
		if !ok {
			return fmt.Errorf("session %s not found", sessionName)
		}

		symbols, err := cmd.Flags().GetStringSlice("symbol")
		if err != nil {
			return err
		}
		if len(symbols) == 0 {
			return fmt.Errorf("--symbol is required")
		}

		channelNames, err := cmd.Flags().GetStringSlice("channels")
		if err != nil {
			return err
		}
		depth, err := cmd.Flags().GetString("depth")
		if err != nil {
			return err
		}
		outDir, err := cmd.Flags().GetString("out")
		if err != nil {
			return err
		}
		duration, err := cmd.Flags().GetDuration("duration")
		if err != nil {
			return err
		}
		uncompressed, err := cmd.Flags().GetBool("uncompressed")
		if err != nil {
			return err
		}

		var channels []types.Channel
		for _, name := range channelNames {
			switch strings.TrimSpace(name) {
			case "book":
				channels = append(channels, types.BookChannel)
			case "trade":
				channels = append(channels, types.MarketTradeChannel)
			case "aggTrade":
				channels = append(channels, types.AggTradeChannel)
			case "bookTicker":
				channels = append(channels, types.BookTickerChannel)
			default:
				return fmt.Errorf("unknown channel %q, want book, trade, aggTrade or bookTicker", name)
			}
		}

		writer, err := replay.NewWriter(replay.WriterConfig{
			Dir:          outDir,
			Exchange:     session.ExchangeName,
			Symbols:      symbols,
			Channels:     channels,
			Uncompressed: uncompressed,
			Note:         fmt.Sprintf("recorded by bbgo marketdata record, session %s", sessionName),
		})
		if err != nil {
			return err
		}
		defer func() {
			if err := writer.Close(); err != nil {
				log.WithError(err).Error("closing the recording")
			}
		}()

		stream := session.Exchange.NewStream()
		stream.SetPublicOnly()

		for _, symbol := range symbols {
			for _, channel := range channels {
				opts := types.SubscribeOptions{}
				if channel == types.BookChannel {
					opts.Depth = types.Depth(depth)
				}
				stream.Subscribe(channel, symbol, opts)
			}
		}

		recorder := replay.NewRecorder(writer, session.ExchangeName)

		// Bind before connecting: StandardStream does not lock its callback
		// slices, so registration has to finish before emission starts.
		recorder.BindStream(stream)

		if err := stream.Connect(ctx); err != nil {
			return err
		}
		defer stream.Close()

		log.Infof("recording %v %v to %s", symbols, channelNames, outDir)

		// Flush periodically so a kill does not cost the whole buffer.
		flushTicker := time.NewTicker(10 * time.Second)
		defer flushTicker.Stop()

		var deadline <-chan time.Time
		if duration > 0 {
			timer := time.NewTimer(duration)
			defer timer.Stop()
			deadline = timer.C
			log.Infof("will stop after %s", duration)
		}

		signals := make(chan os.Signal, 1)
		signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

		for {
			select {
			case <-flushTicker.C:
				if err := writer.Flush(); err != nil {
					log.WithError(err).Warn("flushing the recording")
				}
				log.Infof("%d events recorded, current file %s",
					writer.Count(), filepath.Base(writer.Filename()))

			case <-deadline:
				log.Infof("recorded %d events, %d dropped", writer.Count(), recorder.Dropped())
				return nil

			case sig := <-signals:
				log.Infof("received %s, recorded %d events, %d dropped",
					sig, writer.Count(), recorder.Dropped())
				return nil

			case <-ctx.Done():
				return nil
			}
		}
	},
}

// marketDataDumpSources builds the sources to dump.
//
// There are three ways in, in order of precedence: the sources declared in a
// config file's backtest.dataSources, a recording given with --replay, and the
// archive flags. The config path is the one that exercises what the backtest
// engine will eventually use.
func marketDataDumpSources(cmd *cobra.Command) ([]marketdata.Source, marketdata.Request, error) {
	var empty marketdata.Request

	useConfig, err := cmd.Flags().GetBool("from-config")
	if err != nil {
		return nil, empty, err
	}
	if useConfig {
		return marketDataConfiguredSources(cmd)
	}

	replayPath, err := cmd.Flags().GetString("replay")
	if err != nil {
		return nil, empty, err
	}

	if replayPath == "" {
		src, req, err := marketDataSourceFromFlags(cmd)
		if err != nil {
			return nil, empty, err
		}
		return []marketdata.Source{src}, req, nil
	}

	symbols, err := cmd.Flags().GetStringSlice("symbol")
	if err != nil {
		return nil, empty, err
	}

	src, err := replay.New(replay.Config{Path: replayPath, Symbols: symbols})
	if err != nil {
		return nil, empty, err
	}

	// a recording may have been made moments ago
	since, until, err := marketDataRange(cmd, time.Now().UTC().Add(time.Minute))
	if err != nil {
		return nil, empty, err
	}

	var subs []types.Subscription
	for _, symbol := range symbols {
		for _, channel := range src.Capabilities().Channels {
			subs = append(subs, types.Subscription{Symbol: symbol, Channel: channel})
		}
	}

	return []marketdata.Source{src}, marketdata.Request{
		Since: since, Until: until, Subscriptions: subs,
	}, nil
}

// marketDataConfiguredSources builds the sources declared in the config file's
// backtest.dataSources, which is the shape the backtest engine will read.
func marketDataConfiguredSources(cmd *cobra.Command) ([]marketdata.Source, marketdata.Request, error) {
	var empty marketdata.Request

	if userConfig == nil || userConfig.Backtest == nil {
		return nil, empty, fmt.Errorf("the config file has no backtest section")
	}

	cfgs := userConfig.Backtest.DataSources
	if len(cfgs) == 0 {
		return nil, empty, fmt.Errorf(
			"the config file declares no backtest.dataSources; known source types are %s",
			strings.Join(registry.Types(), ", "))
	}

	cacheDir := userConfig.Backtest.CacheDir
	if flagDir, err := cmd.Flags().GetString("cache-dir"); err == nil && flagDir != "" {
		cacheDir = flagDir
	}
	if cacheDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, empty, err
		}
		cacheDir = filepath.Join(home, ".bbgo", "marketdata")
	}

	sources, err := registry.NewAll(cmd.Context(), cfgs, registry.Options{CacheDir: cacheDir})
	if err != nil {
		return nil, empty, err
	}

	symbols, err := cmd.Flags().GetStringSlice("symbol")
	if err != nil {
		return nil, empty, err
	}
	if len(symbols) == 0 {
		symbols = userConfig.Backtest.Symbols
	}
	if len(symbols) == 0 {
		return nil, empty, fmt.Errorf(
			"no symbols: pass --symbol or set backtest.symbols in the config")
	}

	intervalNames, err := cmd.Flags().GetStringSlice("interval")
	if err != nil {
		return nil, empty, err
	}

	var intervals []types.Interval
	for _, name := range intervalNames {
		if name = strings.TrimSpace(name); name != "" {
			intervals = append(intervals, types.Interval(name))
		}
	}

	since, until, err := marketDataRange(cmd, time.Now().UTC().Truncate(24*time.Hour))
	if err != nil {
		return nil, empty, err
	}

	// Subscribe to the union of what the configured sources can serve, so the
	// command shows everything the configuration makes available rather than
	// requiring the channels to be listed again.
	req := marketdata.Request{
		Since:         since,
		Until:         until,
		Subscriptions: unionSubscriptions(sources, symbols, intervals),
	}

	return sources, req, nil
}

// unionSubscriptions builds one subscription per symbol for every channel any
// configured source serves.
//
// When no interval is given, the intervals the sources declare are used. A kline
// subscription with no interval matches no source that restricts its intervals,
// so defaulting here is what makes `--from-config` work without repeating on the
// command line what the config already says.
func unionSubscriptions(
	sources []marketdata.Source, symbols []string, intervals []types.Interval,
) []types.Subscription {
	var channels []types.Channel
	for _, src := range sources {
		capabilities := src.Capabilities()

		for _, ch := range capabilities.Channels {
			if !slices.Contains(channels, ch) {
				channels = append(channels, ch)
			}
		}

		if len(intervals) == 0 {
			for _, interval := range capabilities.Intervals {
				if !slices.Contains(intervals, interval) {
					intervals = append(intervals, interval)
				}
			}
		}
	}

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

	// archives exist only for completed UTC days
	since, until, err := marketDataRange(cmd, time.Now().UTC().Truncate(24*time.Hour))
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

// marketDataRange resolves --since and --until.
//
// The default end differs by source kind. Archives are published per completed
// UTC day, so defaulting to today's midnight avoids requesting a file that does
// not exist yet; a recording can have been made minutes ago, so there the
// default is now.
func marketDataRange(cmd *cobra.Command, defaultUntil time.Time) (since, until time.Time, err error) {
	sinceStr, err := cmd.Flags().GetString("since")
	if err != nil {
		return since, until, err
	}
	untilStr, err := cmd.Flags().GetString("until")
	if err != nil {
		return since, until, err
	}

	if sinceStr == "" {
		// A recording carries its own time range, so replaying all of it is a
		// sensible default; archives need an explicit start and mark --since
		// required.
		since = time.Unix(0, 0).UTC()
	} else if since, err = parseMarketDataTime(sinceStr); err != nil {
		return since, until, fmt.Errorf("bad --since: %w", err)
	}

	if untilStr == "" {
		until = defaultUntil
	} else if until, err = parseMarketDataTime(untilStr); err != nil {
		return since, until, fmt.Errorf("bad --until: %w", err)
	}

	if !until.After(since) {
		return since, until, fmt.Errorf(
			"--until (%s) must be after --since (%s)",
			until.Format(time.RFC3339), since.Format(time.RFC3339))
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

	}

	// Only the download command needs these: dump can take both from the config
	// file, and a recording carries its own range.
	for _, name := range []string{"symbol", "since"} {
		if err := marketDataDownloadCmd.MarkFlagRequired(name); err != nil {
			panic(err)
		}
	}

	marketDataDownloadCmd.Flags().Bool("dry-run", false, "print the archive URLs without downloading")
	marketDataDumpCmd.Flags().Bool("from-config", false,
		"build the sources from the config file's backtest.dataSources")
	marketDataDumpCmd.Flags().String("replay", "",
		"replay a recording directory or file instead of reading archives")
	marketDataDumpCmd.Flags().Bool("check-book", false,
		"apply book events to an order book and verify the sequence is contiguous")
	marketDataDumpCmd.Flags().Int("limit", 20, "print at most this many events, 0 for all")
	marketDataDumpCmd.Flags().Bool("count-only", false, "only count events, do not print them")

	marketDataRecordCmd.Flags().String("session", "", "exchange session to record from")
	marketDataRecordCmd.Flags().StringSlice("symbol", nil, "symbols to record, e.g. BTCUSDT")
	marketDataRecordCmd.Flags().StringSlice("channels", []string{"book", "trade"},
		"channels to record: book, trade, aggTrade, bookTicker")
	marketDataRecordCmd.Flags().String("depth", "full", "order book depth: full, medium, 1, 5 or 20")
	marketDataRecordCmd.Flags().String("out", "./recordings", "output directory")
	marketDataRecordCmd.Flags().Duration("duration", 0, "stop after this long, 0 to run until interrupted")
	marketDataRecordCmd.Flags().Bool("uncompressed", false, "write plain .jsonl instead of .jsonl.gz")

	marketDataCmd.AddCommand(marketDataDownloadCmd)
	marketDataCmd.AddCommand(marketDataRecordCmd)
	marketDataCmd.AddCommand(marketDataDumpCmd)
	RootCmd.AddCommand(marketDataCmd)
}
