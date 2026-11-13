package grpcsource

import (
	"context"
	"fmt"
	"slices"
	"time"

	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/c9s/bbgo/pkg/marketdata"
	"github.com/c9s/bbgo/pkg/marketdata/pb"
	"github.com/c9s/bbgo/pkg/types"
)

func nanoTime(ns int64) time.Time { return time.Unix(0, ns).UTC() }

// DefaultBufferSize is the cursor's event buffer. It is smaller than a file
// source's, because backpressure here is genuinely useful: the receive goroutine
// blocking stops it calling Recv, which closes the HTTP/2 window and stops the
// server sending. Buffering more would just move memory pressure to the client.
const DefaultBufferSize = 256

// Config configures a gRPC replay source.
type Config struct {
	// Address is the server's host:port.
	Address string `json:"address" yaml:"address"`

	// Name overrides the source name in logs and merge diagnostics.
	Name string `json:"name,omitempty" yaml:"name,omitempty"`

	// Insecure uses a plaintext connection. It defaults to true, since the
	// expected deployment is a replay server on a trusted network; set TLS to
	// override.
	TLS bool `json:"tls,omitempty" yaml:"tls,omitempty"`

	// DialTimeout bounds the initial connection and the Capability call.
	DialTimeout time.Duration `json:"dialTimeout,omitempty" yaml:"dialTimeout,omitempty"`

	// BufferSize overrides DefaultBufferSize.
	BufferSize int `json:"bufferSize,omitempty" yaml:"bufferSize,omitempty"`

	// Speed paces the stream as a multiple of real time. Zero, the default,
	// means as fast as the transport allows.
	Speed float64 `json:"speed,omitempty" yaml:"speed,omitempty"`
}

// Source replays market data from a remote bbgo node.
type Source struct {
	cfg    Config
	conn   *grpc.ClientConn
	client pb.MarketDataServiceClient

	capability marketdata.Capability
}

var _ marketdata.Source = (*Source)(nil)

// New dials the server and queries its capability, so a merge can reject an
// unusable source before it starts reading.
func New(ctx context.Context, cfg Config) (*Source, error) {
	if cfg.Address == "" {
		return nil, fmt.Errorf("grpcsource: address is required")
	}
	if cfg.DialTimeout <= 0 {
		cfg.DialTimeout = 10 * time.Second
	}

	var opts []grpc.DialOption
	if !cfg.TLS {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	// grpc.NewClient rather than the deprecated grpc.Dial: it does not block on
	// the initial connection, which the Capability call below does for us.
	conn, err := grpc.NewClient(cfg.Address, opts...)
	if err != nil {
		return nil, fmt.Errorf("grpcsource: dialing %s: %w", cfg.Address, err)
	}

	s := &Source{cfg: cfg, conn: conn, client: pb.NewMarketDataServiceClient(conn)}

	capCtx, cancel := context.WithTimeout(ctx, cfg.DialTimeout)
	defer cancel()

	if err := s.loadCapability(capCtx); err != nil {
		conn.Close()
		return nil, err
	}

	return s, nil
}

// NewWithConn builds a Source over an existing connection, for tests using an
// in-process listener.
func NewWithConn(ctx context.Context, cfg Config, conn *grpc.ClientConn) (*Source, error) {
	s := &Source{cfg: cfg, conn: conn, client: pb.NewMarketDataServiceClient(conn)}
	if err := s.loadCapability(ctx); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Source) loadCapability(ctx context.Context) error {
	resp, err := s.client.Capability(ctx, &pb.CapabilityRequest{})
	if err != nil {
		return fmt.Errorf("grpcsource: querying capability of %s: %w", s.cfg.Address, err)
	}

	cap := marketdata.Capability{HasHistory: resp.HasHistory}

	for _, name := range resp.Exchanges {
		cap.Exchanges = append(cap.Exchanges, types.ExchangeName(name))
	}
	for _, name := range resp.Channels {
		cap.Channels = append(cap.Channels, types.Channel(name))
	}
	for _, name := range resp.Intervals {
		cap.Intervals = append(cap.Intervals, types.Interval(name))
	}
	cap.Symbols = slices.Clone(resp.Symbols)

	if resp.CoverageStartNs != 0 {
		cap.CoverageStart = nanoTime(resp.CoverageStartNs)
	}
	if resp.CoverageEndNs != 0 {
		cap.CoverageEnd = nanoTime(resp.CoverageEndNs)
	}

	s.capability = cap
	return nil
}

func (s *Source) Name() string {
	if s.cfg.Name != "" {
		return s.cfg.Name
	}
	return "grpc/" + s.cfg.Address
}

func (s *Source) Capabilities() marketdata.Capability { return s.capability }

// Close releases the connection.
func (s *Source) Close() error {
	if s.conn == nil {
		return nil
	}
	return s.conn.Close()
}

func (s *Source) Open(ctx context.Context, req marketdata.Request) (marketdata.Cursor, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	if err := s.capability.Validate(s.Name(), req); err != nil {
		return nil, err
	}

	subs := make([]*pb.Subscription, 0, len(req.Subscriptions))
	for _, sub := range req.Subscriptions {
		subs = append(subs, &pb.Subscription{
			Channel:  string(sub.Channel),
			Symbol:   sub.Symbol,
			Interval: string(sub.Options.Interval),
			Depth:    string(sub.Options.Depth),
		})
	}

	stream, err := s.client.Replay(ctx, &pb.ReplayRequest{
		Subscriptions: subs,
		SinceNs:       req.Since.UnixNano(),
		UntilNs:       req.Until.UnixNano(),
		Speed:         s.cfg.Speed,
	})
	if err != nil {
		return nil, fmt.Errorf("grpcsource: starting replay: %w", err)
	}

	bufSize := s.cfg.BufferSize
	if bufSize <= 0 {
		bufSize = DefaultBufferSize
	}

	cursor := marketdata.NewChanCursor(ctx, bufSize)
	logger := log.WithField("component", s.Name())

	// The receive loop is the producer. It blocks in Push when the consumer is
	// behind, which stops it calling Recv, which lets HTTP/2 flow control push
	// back to the server — so no buffering policy is needed beyond the default.
	go func() {
		for {
			msg, err := stream.Recv()
			if err != nil {
				if isStreamEnd(err) {
					cursor.Finish()
				} else {
					cursor.Fail(fmt.Errorf("grpcsource: receiving from %s: %w", s.cfg.Address, err))
				}
				return
			}

			ev, err := FromProto(msg)
			if err != nil {
				cursor.Fail(err)
				return
			}

			if !cursor.Push(&ev) {
				logger.Debug("consumer closed the cursor, stopping the receive loop")
				return
			}
		}
	}()

	return cursor, nil
}
