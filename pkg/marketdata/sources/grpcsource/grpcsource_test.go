package grpcsource_test

import (
	"context"
	"errors"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	"github.com/c9s/bbgo/pkg/marketdata"
	"github.com/c9s/bbgo/pkg/marketdata/mdtest"
	"github.com/c9s/bbgo/pkg/marketdata/pb"
	"github.com/c9s/bbgo/pkg/marketdata/sources/grpcsource"
	"github.com/c9s/bbgo/pkg/types"
)

// fakeServer scripts a Replay stream, so the client is exercised over a real
// gRPC connection without needing a server implementation.
type fakeServer struct {
	pb.UnimplementedMarketDataServiceServer

	events   []*pb.Event
	failAt   int // send this many events, then fail; -1 to never fail
	capResp  *pb.CapabilityResponse
	sendGate chan struct{} // when non-nil, each send waits for a token
	sent     chan int      // receives the count after each send
}

func (s *fakeServer) Capability(
	ctx context.Context, _ *pb.CapabilityRequest,
) (*pb.CapabilityResponse, error) {
	if s.capResp != nil {
		return s.capResp, nil
	}
	return &pb.CapabilityResponse{
		Exchanges:  []string{"binance"},
		Channels:   []string{"book", "trade", "kline"},
		Symbols:    []string{"BTCUSDT"},
		HasHistory: true,
	}, nil
}

func (s *fakeServer) Replay(req *pb.ReplayRequest, stream pb.MarketDataService_ReplayServer) error {
	for i, ev := range s.events {
		if s.failAt >= 0 && i == s.failAt {
			return errors.New("server exploded")
		}
		if s.sendGate != nil {
			select {
			case <-s.sendGate:
			case <-stream.Context().Done():
				return stream.Context().Err()
			}
		}
		if err := stream.Send(ev); err != nil {
			return err
		}
		if s.sent != nil {
			s.sent <- i + 1
		}
	}
	return nil
}

// startServer runs srv on an in-process listener and returns a connected client.
func startServer(t *testing.T, srv *fakeServer) *grpc.ClientConn {
	t.Helper()

	lis := bufconn.Listen(1024 * 1024)
	grpcServer := grpc.NewServer()
	pb.RegisterMarketDataServiceServer(grpcServer, srv)

	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			// Serve returns when the listener closes; nothing to assert.
			_ = err
		}
	}()

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)

	t.Cleanup(func() {
		conn.Close()
		grpcServer.Stop()
		lis.Close()
	})

	return conn
}

func tradeProto(ms int64, id uint64) *pb.Event {
	return &pb.Event{
		Type:        pb.EventType_EVENT_TYPE_TRADE,
		Exchange:    "binance",
		Symbol:      "BTCUSDT",
		EventTimeNs: ms * 1e6,
		Sequence:    id,
		Payload: &pb.Event_Trade{Trade: &pb.Trade{
			Id: fmt.Sprintf("%d", id), Price: "100.5", Quantity: "0.25",
			QuoteQuantity: "25.125", IsBuyer: true,
		}},
	}
}

func fullRequest() marketdata.Request {
	return marketdata.Request{
		Since: time.UnixMilli(0),
		Until: time.UnixMilli(1_000_000),
		Subscriptions: []types.Subscription{
			{Symbol: "BTCUSDT", Channel: types.MarketTradeChannel},
			{Symbol: "BTCUSDT", Channel: types.BookChannel},
		},
	}
}

func TestSource_Replay(t *testing.T) {
	ctx := context.Background()

	conn := startServer(t, &fakeServer{
		failAt: -1,
		events: []*pb.Event{tradeProto(1000, 1), tradeProto(2000, 2), tradeProto(3000, 3)},
	})

	src, err := grpcsource.NewWithConn(ctx, grpcsource.Config{Name: "remote"}, conn)
	require.NoError(t, err)

	assert.True(t, src.Capabilities().HasHistory)
	assert.Contains(t, src.Capabilities().Channels, types.BookChannel)

	cur, err := src.Open(ctx, fullRequest())
	require.NoError(t, err)
	defer cur.Close()

	got := mdtest.Collect(t, cur)
	require.Len(t, got, 3)

	assert.Equal(t, int64(1000), got[0].Key.TimeNs/1e6)
	assert.Equal(t, uint64(1), got[0].Key.Seq)
	require.NotNil(t, got[0].Trade)
	assert.Equal(t, "100.5", got[0].Trade.Price.String())
	assert.Equal(t, "25.125", got[0].Trade.QuoteQuantity.String(),
		"the published quote quantity must be preferred over the product")

	for i := 1; i < len(got); i++ {
		assert.LessOrEqual(t, got[i-1].Key.Compare(got[i].Key), 0)
	}
}

func TestSource_ServerErrorReachesErr(t *testing.T) {
	ctx := context.Background()

	conn := startServer(t, &fakeServer{
		failAt: 2,
		events: []*pb.Event{tradeProto(1000, 1), tradeProto(2000, 2), tradeProto(3000, 3)},
	})

	src, err := grpcsource.NewWithConn(ctx, grpcsource.Config{}, conn)
	require.NoError(t, err)

	cur, err := src.Open(ctx, fullRequest())
	require.NoError(t, err)
	defer cur.Close()

	var count int
	for cur.Next() {
		count++
	}

	assert.Equal(t, 2, count)
	require.Error(t, cur.Err())
	assert.Contains(t, cur.Err().Error(), "server exploded")
}

func TestSource_ContextCancelIsCleanNotAnError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	conn := startServer(t, &fakeServer{
		failAt: -1,
		events: []*pb.Event{tradeProto(1000, 1), tradeProto(2000, 2)},
	})

	src, err := grpcsource.NewWithConn(context.Background(), grpcsource.Config{}, conn)
	require.NoError(t, err)

	cur, err := src.Open(ctx, fullRequest())
	require.NoError(t, err)
	defer cur.Close()

	cancel()

	for cur.Next() {
	}
	assert.ErrorIs(t, cur.Err(), context.Canceled)
}

// TestSource_BackpressureStallsTheServer is the point of routing the receive
// loop through a ChanCursor: a consumer that stops reading must stop the server
// sending, rather than the client buffering the whole stream.
//
// The events have to be large for this to be observable at all. gRPC's default
// per-stream HTTP/2 window is 64 KiB, so a few dozen small events complete
// regardless of the client's buffer; the stall only appears once the volume in
// flight exceeds the window.
func TestSource_BackpressureStallsTheServer(t *testing.T) {
	ctx := context.Background()

	const total = 200
	events := make([]*pb.Event, total)
	for i := range events {
		events[i] = bigBookProto(int64(i+1)*1000, uint64(i+1), 200)
	}

	gate := make(chan struct{}, total)
	sent := make(chan int, total)

	conn := startServer(t, &fakeServer{failAt: -1, events: events, sendGate: gate, sent: sent})

	src, err := grpcsource.NewWithConn(ctx, grpcsource.Config{BufferSize: 4}, conn)
	require.NoError(t, err)

	cur, err := src.Open(ctx, fullRequest())
	require.NoError(t, err)
	defer cur.Close()

	// Allow the server to send everything it can, without consuming any of it.
	for i := 0; i < total; i++ {
		gate <- struct{}{}
	}
	time.Sleep(200 * time.Millisecond)

	sentCount := drain(sent)
	assert.Less(t, sentCount, total,
		"a consumer that never reads must stall the server (sent %d of %d)", sentCount, total)

	// Draining now must still yield every event: backpressure delays, it never drops.
	got := mdtest.Collect(t, cur)
	assert.Len(t, got, total)
}

// drain empties ch and returns the last value seen, or zero.
func drain(ch <-chan int) int {
	var last int
	for {
		select {
		case v := <-ch:
			last = v
		default:
			return last
		}
	}
}

// bigBookProto builds a book event with levels levels per side, so a stream of
// them exceeds the HTTP/2 flow-control window.
func bigBookProto(ms int64, seq uint64, levels int) *pb.Event {
	bids := make([]*pb.PriceVolume, levels)
	asks := make([]*pb.PriceVolume, levels)
	for i := 0; i < levels; i++ {
		bids[i] = &pb.PriceVolume{
			Price:  fmt.Sprintf("%d.12345678", 90000-i),
			Volume: fmt.Sprintf("%d.87654321", i+1),
		}
		asks[i] = &pb.PriceVolume{
			Price:  fmt.Sprintf("%d.12345678", 90001+i),
			Volume: fmt.Sprintf("%d.87654321", i+1),
		}
	}

	return &pb.Event{
		Type:        pb.EventType_EVENT_TYPE_BOOK_UPDATE,
		Exchange:    "binance",
		Symbol:      "BTCUSDT",
		EventTimeNs: ms * 1e6,
		Sequence:    seq,
		Payload: &pb.Event_Book{Book: &pb.Book{
			Bids: bids, Asks: asks, LastUpdateId: int64(seq),
		}},
	}
}

func TestSource_RejectsLiveOnlyServer(t *testing.T) {
	ctx := context.Background()

	conn := startServer(t, &fakeServer{
		failAt: -1,
		capResp: &pb.CapabilityResponse{
			Channels:   []string{"trade"},
			HasHistory: false,
		},
	})

	src, err := grpcsource.NewWithConn(ctx, grpcsource.Config{}, conn)
	require.NoError(t, err)

	_, err = src.Open(ctx, fullRequest())
	require.Error(t, err)

	var unsupported *marketdata.UnsupportedError
	require.ErrorAs(t, err, &unsupported)
	assert.Contains(t, unsupported.Reason, "live-only")
}

func TestFromProto_RejectsMissingTimestamp(t *testing.T) {
	ev := tradeProto(1000, 1)
	ev.EventTimeNs = 0

	_, err := grpcsource.FromProto(ev)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no event_time_ns",
		"a zero timestamp would sort before every other event, so it must be rejected")
}

func TestFromProto_RejectsMissingPayload(t *testing.T) {
	_, err := grpcsource.FromProto(&pb.Event{
		Type: pb.EventType_EVENT_TYPE_TRADE, Symbol: "BTCUSDT", EventTimeNs: 1000,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no payload")
}

// TestProtoRoundTrip checks that the converters agree, which is what keeps a
// future server implementation honest against this client.
func TestProtoRoundTrip(t *testing.T) {
	events := mdtest.Events(t, `
		t=1000 bookSnapshot seq=100 bids=100,10;99,5 asks=101,10
		t=2000 bookUpdate   seq=101 bids=100,0
		t=3000 trade price=100.5 qty=0.25 seq=7001
		t=60000 kline interval=1m o=100 h=102 l=99 c=101 v=12.5
		t=60100 bookTicker seq=102 bid=100.1,3 ask=100.2,4
	`)

	for i := range events {
		want := events[i]
		want.PrevSeq = want.Key.Seq - 1

		wire, err := grpcsource.ToProto(&want)
		require.NoError(t, err, "%s", want.Type)

		got, err := grpcsource.FromProto(wire)
		require.NoError(t, err, "%s", want.Type)

		assert.Equal(t, want.Type, got.Type)
		assert.Equal(t, want.Key.TimeNs, got.Key.TimeNs)
		assert.Equal(t, want.Key.Rank, got.Key.Rank, "%s rank", want.Type)
		assert.Equal(t, want.Key.Seq, got.Key.Seq)
		assert.Equal(t, want.PrevSeq, got.PrevSeq, "%s prevSeq", want.Type)
		assert.Equal(t, want.Symbol, got.Symbol)

		switch want.Type {
		case marketdata.EventTypeBookSnapshot, marketdata.EventTypeBookUpdate:
			require.NotNil(t, got.Book)
			assert.Equal(t, len(want.Book.Bids), len(got.Book.Bids))
			if len(want.Book.Bids) > 0 {
				assert.Equal(t, want.Book.Bids[0].Price.String(), got.Book.Bids[0].Price.String())
				assert.Equal(t, want.Book.Bids[0].Volume.String(), got.Book.Bids[0].Volume.String(),
					"a zero volume must survive: it means removal")
			}
		case marketdata.EventTypeTrade:
			require.NotNil(t, got.Trade)
			assert.Equal(t, want.Trade.Price.String(), got.Trade.Price.String())
		case marketdata.EventTypeKLine:
			require.NotNil(t, got.KLine)
			assert.Equal(t, want.KLine.Interval, got.KLine.Interval)
			assert.Equal(t, want.KLine.Close.String(), got.KLine.Close.String())
			assert.Equal(t, want.KLine.StartTime.Time().UnixNano(), got.KLine.StartTime.Time().UnixNano())
		case marketdata.EventTypeBookTicker:
			require.NotNil(t, got.BookTicker)
			assert.Equal(t, want.BookTicker.Buy.String(), got.BookTicker.Buy.String())
		}
	}
}
