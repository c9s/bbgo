package amberdata

import (
	"context"
	"fmt"
	"slices"
	"time"

	log "github.com/sirupsen/logrus"
	"golang.org/x/time/rate"

	"github.com/c9s/bbgo/pkg/marketdata"
	"github.com/c9s/bbgo/pkg/marketdata/sources/amberdata/amberdataapi"
	"github.com/c9s/bbgo/pkg/types"
)

// Config configures an AmberData source.
type Config struct {
	// APIKey is required unless a Client is supplied. Its prefix determines the
	// documented rate limits, which the source applies automatically.
	APIKey string `json:"apiKey,omitempty" yaml:"apiKey,omitempty"`

	// Exchange is the venue to read, and exactly one is allowed per request.
	Exchange string `json:"exchange" yaml:"exchange"`

	// AssetClass selects the futures or spot endpoints. Defaults to futures.
	AssetClass amberdataapi.AssetClass `json:"assetClass,omitempty" yaml:"assetClass,omitempty"`

	// Symbols restricts the source. Empty serves whatever is requested.
	Symbols []string `json:"symbols,omitempty" yaml:"symbols,omitempty"`

	// MaxLevel caps order book depth. Zero leaves it to the server.
	MaxLevel int `json:"maxLevel,omitempty" yaml:"maxLevel,omitempty"`

	// Chunk is the initial request window. It shrinks automatically when the
	// server reports the result too large.
	Chunk time.Duration `json:"chunk,omitempty" yaml:"chunk,omitempty"`

	// MinChunk is the floor below which shrinking gives up.
	MinChunk time.Duration `json:"minChunk,omitempty" yaml:"minChunk,omitempty"`

	// RequestsPerSecond overrides the rate derived from the key prefix.
	RequestsPerSecond float64 `json:"requestsPerSecond,omitempty" yaml:"requestsPerSecond,omitempty"`

	// BufferSize is the cursor's event buffer.
	BufferSize int `json:"bufferSize,omitempty" yaml:"bufferSize,omitempty"`

	// Name overrides the source name in logs and merge diagnostics.
	Name string `json:"name,omitempty" yaml:"name,omitempty"`

	// Client overrides the REST client, for tests.
	Client Client `json:"-" yaml:"-"`
}

func (c *Config) applyDefaults() {
	if c.AssetClass == "" {
		c.AssetClass = amberdataapi.AssetClassFutures
	}
	if c.Chunk <= 0 {
		c.Chunk = time.Hour
	}
	if c.MinChunk <= 0 {
		c.MinChunk = time.Minute
	}
	if c.BufferSize <= 0 {
		c.BufferSize = 1024
	}

	if c.RequestsPerSecond <= 0 {
		// The documented limit for the key's tier, halved, so a backfill leaves
		// room for whatever else is using the same key.
		if tier, ok := amberdataapi.TierFromKey(c.APIKey); ok {
			c.RequestsPerSecond = tier.RequestsPerSec / 2
		} else {
			c.RequestsPerSecond = 5
		}
	}
}

func (c *Config) validate() error {
	if c.Exchange == "" {
		return fmt.Errorf("amberdata: exchange is required, and only one is allowed per request")
	}
	if c.APIKey == "" && c.Client == nil {
		return fmt.Errorf("amberdata: apiKey is required")
	}
	switch c.AssetClass {
	case amberdataapi.AssetClassFutures, amberdataapi.AssetClassSpot:
	default:
		return fmt.Errorf("amberdata: unknown asset class %q, want futures or spot", c.AssetClass)
	}
	return nil
}

// Source reads historical market data from AmberData.
type Source struct {
	cfg     Config
	client  Client
	limiter *rate.Limiter
}

var _ marketdata.Source = (*Source)(nil)

// New builds a Source from cfg.
func New(cfg Config) (*Source, error) {
	cfg.applyDefaults()
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	client := cfg.Client
	if client == nil {
		client = NewClient(cfg.APIKey, cfg.AssetClass)
	}

	return &Source{
		cfg:     cfg,
		client:  client,
		limiter: rate.NewLimiter(rate.Limit(cfg.RequestsPerSecond), 1),
	}, nil
}

func (s *Source) Name() string {
	if s.cfg.Name != "" {
		return s.cfg.Name
	}
	return fmt.Sprintf("amberdata/%s/%s", s.cfg.Exchange, s.cfg.AssetClass)
}

// Capabilities reports what this source serves.
//
// Unlike the CSV archives it does declare BookChannel: order-book-snapshots and
// order-book-events are the reason this provider exists.
func (s *Source) Capabilities() marketdata.Capability {
	return marketdata.Capability{
		Exchanges: []types.ExchangeName{types.ExchangeName(s.cfg.Exchange)},
		Channels: []types.Channel{
			types.BookChannel,
			types.MarketTradeChannel,
			types.AggTradeChannel,
			types.KLineChannel,
		},
		Symbols:    s.cfg.Symbols,
		HasHistory: true,
	}
}

func (s *Source) Open(ctx context.Context, req marketdata.Request) (marketdata.Cursor, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	if err := s.Capabilities().Validate(s.Name(), req); err != nil {
		return nil, err
	}

	plan, err := s.plan(req)
	if err != nil {
		return nil, err
	}
	if len(plan) == 0 {
		return nil, &marketdata.UnsupportedError{
			Source: s.Name(),
			Reason: "no requested subscription maps to an AmberData endpoint",
		}
	}

	// Each stream is fetched independently and the results merged, because the
	// API has no combined endpoint: trades, snapshots and events are separate
	// paginated queries whose pages interleave in time.
	cursors := make([]marketdata.NamedCursor, 0, len(plan))
	for _, st := range plan {
		cursors = append(cursors, marketdata.NamedCursor{
			Name:   s.Name() + "/" + st.name(),
			Cursor: s.openStream(ctx, st, req),
		})
	}

	if len(cursors) == 1 {
		return cursors[0].Cursor, nil
	}
	return marketdata.Merge(cursors), nil
}

// streamKind is one AmberData endpoint applied to one instrument.
type streamKind struct {
	endpoint   string // trades, orderBookSnapshots, orderBookEvents, ohlcv
	instrument string
	symbol     string
	interval   types.Interval
}

func (k streamKind) name() string {
	if len(k.interval) > 0 {
		return fmt.Sprintf("%s/%s/%s", k.endpoint, k.symbol, k.interval)
	}
	return fmt.Sprintf("%s/%s", k.endpoint, k.symbol)
}

// plan maps the requested subscriptions onto endpoints.
//
// A book subscription becomes two streams: the snapshots endpoint establishes
// the base state and the events endpoint supplies the diffs. There is no single
// endpoint that returns both, and an events stream on its own cannot initialize
// a book.
func (s *Source) plan(req marketdata.Request) ([]streamKind, error) {
	var out []streamKind

	add := func(k streamKind) {
		if !slices.Contains(out, k) {
			out = append(out, k)
		}
	}

	for _, sub := range req.Subscriptions {
		if len(s.cfg.Symbols) > 0 && !slices.Contains(s.cfg.Symbols, sub.Symbol) {
			continue
		}

		instrument := amberdataapi.NormalizeInstrument(s.cfg.AssetClass, sub.Symbol)

		switch sub.Channel {
		case types.BookChannel:
			add(streamKind{endpoint: "orderBookSnapshots", instrument: instrument, symbol: sub.Symbol})
			add(streamKind{endpoint: "orderBookEvents", instrument: instrument, symbol: sub.Symbol})

		case types.MarketTradeChannel, types.AggTradeChannel:
			add(streamKind{endpoint: "trades", instrument: instrument, symbol: sub.Symbol})

		case types.KLineChannel:
			interval := sub.Options.Interval
			if _, ok := ohlcvInterval(interval.Duration()); !ok {
				return nil, &marketdata.UnsupportedError{
					Source:   s.Name(),
					Channel:  sub.Channel,
					Interval: interval,
					Reason: "the ohlcv endpoint offers only minute, hour and day buckets, " +
						"so this interval has no equivalent",
				}
			}
			add(streamKind{
				endpoint: "ohlcv", instrument: instrument, symbol: sub.Symbol, interval: interval,
			})
		}
	}

	return out, nil
}

// openStream starts one endpoint's pagination in the background, feeding a
// cursor. The producer blocks when the consumer is behind, which throttles the
// pagination itself rather than buffering a range in memory.
func (s *Source) openStream(
	ctx context.Context, kind streamKind, req marketdata.Request,
) marketdata.Cursor {
	cursor := marketdata.NewChanCursor(ctx, s.cfg.BufferSize)
	logger := log.WithField("component", s.Name()+"/"+kind.name())

	go func() {
		err := s.runStream(cursor.Context(), cursor, kind, req, logger)
		switch {
		case err == nil:
			cursor.Finish()
		case cursor.Context().Err() != nil:
			// The consumer closed the cursor; not a failure.
			cursor.Finish()
		default:
			cursor.Fail(err)
		}
	}()

	return cursor
}

func (s *Source) runStream(
	ctx context.Context,
	cursor *marketdata.ChanCursor,
	kind streamKind,
	req marketdata.Request,
	logger *log.Entry,
) error {
	query := amberdataapi.Query{
		Exchange:   s.cfg.Exchange,
		Instrument: kind.instrument,
		MaxLevel:   s.cfg.MaxLevel,
	}

	exchange := types.ExchangeName(s.cfg.Exchange)

	switch kind.endpoint {
	case "trades":
		w := &pageWalker[amberdataapi.Trade]{
			fetch: func(ctx context.Context, since, until time.Time, next string) (amberdataapi.Page[amberdataapi.Trade], error) {
				q := query
				q.Since, q.Until = since, until
				return s.client.GetTrades(ctx, q, next)
			},
			timeOf:    func(t amberdataapi.Trade) int64 { return t.EventTimeNano() },
			idOf:      func(t amberdataapi.Trade) string { return t.TradeID },
			limiter:   s.limiter,
			chunk:     s.cfg.Chunk,
			minChunk:  s.cfg.MinChunk,
			maxWindow: amberdataapi.MaxTradesWindow,
			logger:    logger,
		}

		return w.walk(ctx, req.Since, req.Until, func(t amberdataapi.Trade) bool {
			ev := tradeToEvent(t, exchange, kind.symbol)
			return cursor.Push(&ev)
		})

	case "orderBookSnapshots", "orderBookEvents":
		snapshot := kind.endpoint == "orderBookSnapshots"

		w := &pageWalker[amberdataapi.Book]{
			fetch: func(ctx context.Context, since, until time.Time, next string) (amberdataapi.Page[amberdataapi.Book], error) {
				q := query
				q.Since, q.Until = since, until
				if snapshot {
					return s.client.GetOrderBookSnapshots(ctx, q, next)
				}
				return s.client.GetOrderBookEvents(ctx, q, next)
			},
			timeOf: func(b amberdataapi.Book) int64 { return b.EventTimeNano() },
			idOf: func(b amberdataapi.Book) string {
				if b.Sequence.Valid {
					return fmt.Sprintf("%d", b.Sequence.Value)
				}
				return ""
			},
			limiter:   s.limiter,
			chunk:     s.cfg.Chunk,
			minChunk:  s.cfg.MinChunk,
			maxWindow: amberdataapi.MaxOrderBookWindow,
			logger:    logger,
		}

		return w.walk(ctx, req.Since, req.Until, func(b amberdataapi.Book) bool {
			ev := bookToEvent(b, exchange, kind.symbol, snapshot, s.cfg.MaxLevel > 0)
			return cursor.Push(&ev)
		})

	case "ohlcv":
		bucket, ok := ohlcvInterval(kind.interval.Duration())
		if !ok {
			return fmt.Errorf("amberdata: interval %s has no ohlcv bucket", kind.interval)
		}
		query.TimeInterval = bucket

		w := &pageWalker[amberdataapi.OHLCV]{
			fetch: func(ctx context.Context, since, until time.Time, next string) (amberdataapi.Page[amberdataapi.OHLCV], error) {
				q := query
				q.Since, q.Until = since, until
				return s.client.GetOHLCV(ctx, q, next)
			},
			timeOf:   func(o amberdataapi.OHLCV) int64 { return o.ExchangeTimestamp.UnixNano() },
			idOf:     func(o amberdataapi.OHLCV) string { return "" },
			limiter:  s.limiter,
			chunk:    s.cfg.Chunk,
			minChunk: s.cfg.MinChunk,
			logger:   logger,
		}

		return w.walk(ctx, req.Since, req.Until, func(o amberdataapi.OHLCV) bool {
			ev := ohlcvToEvent(o, exchange, kind.symbol, kind.interval)
			return cursor.Push(&ev)
		})

	default:
		return fmt.Errorf("amberdata: unknown endpoint %q", kind.endpoint)
	}
}
