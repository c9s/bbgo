package backpackapi

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/testutil"
)

// publicMessageRecordFile holds a recorded public session.
//
// The depth stream dominates a live recording, so the committed file keeps only the first few
// hundred depth frames; the run that survives is contiguous, so the update ids still line up for
// anything that wants to replay the order book. Every rarer event type is kept in full.
const publicMessageRecordFile = "testdata/backpack_ws_raw.jsonl"

const privateMessageRecordFile = "testdata/backpack_ws_private_raw.jsonl"

// recordWebSocketMessages dials the websocket, sends the given subscribe requests and appends
// every received frame to recordFile as one JSON object per line.
func recordWebSocketMessages(
	t *testing.T, recordFile string, duration time.Duration, requests []WebsocketRequest,
) {
	c, _, err := websocket.DefaultDialer.Dial(WebSocketURL, nil)
	if err != nil {
		t.Fatalf("failed to connect to the backpack websocket: %v", err)
	}

	defer c.Close()

	file, err := os.Create(recordFile)
	if err != nil {
		t.Fatalf("failed to create the record file: %v", err)
	}

	defer func() {
		if err := file.Sync(); err != nil {
			logrus.Errorf("failed to sync the record file: %v", err)
		}

		if err := file.Close(); err != nil {
			logrus.Errorf("failed to close the record file: %v", err)
		}
	}()

	for _, req := range requests {
		if err := c.WriteJSON(req); err != nil {
			t.Fatalf("failed to send the subscribe request %+v: %v", req, err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	var received int
	for {
		select {
		case <-ctx.Done():
			t.Logf("recorded %d messages to %s", received, recordFile)
			return

		default:
			if err := c.SetReadDeadline(time.Now().Add(30 * time.Second)); err != nil {
				logrus.Errorf("failed to set the read deadline: %v", err)
				return
			}

			_, msg, err := c.ReadMessage()
			if err != nil {
				if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					t.Log("the websocket closed normally")
					return
				}

				logrus.Errorf("read error: %v", err)
				t.Logf("recorded %d messages to %s", received, recordFile)
				return
			}

			if _, err := file.Write(append(msg, '\n')); err != nil {
				logrus.Errorf("failed to write the message: %v", err)
			}

			received++
			t.Logf("received: %s", string(msg))
		}
	}
}

// TestWebSocketRecord captures the public streams. Run it to refresh the test data:
//
//	TEST_BACKPACK_WS_RECORD=1 go test -v ./pkg/exchange/backpack/backpackapi/ -run TestWebSocketRecord
func TestWebSocketRecord(t *testing.T) {
	if os.Getenv("TEST_BACKPACK_WS_RECORD") == "" {
		t.Skip("TEST_BACKPACK_WS_RECORD is not set, skipping the live websocket recording")
	}

	recordWebSocketMessages(t, publicMessageRecordFile, 90*time.Second, []WebsocketRequest{
		{Method: WebsocketMethodSubscribe, Params: []string{
			"bookTicker.SOL_USDC",
			"trade.SOL_USDC",
			"depth.SOL_USDC",
			"ticker.SOL_USDC",
			"kline.1m.SOL_USDC",
			"markPrice.SOL_USDC_PERP",
		}},
	})
}

// TestWebSocketRecordPrivate captures the private streams. It needs credentials:
//
//	TEST_BACKPACK=1 TEST_BACKPACK_WS_RECORD=1 dotenv -f .env.local -- \
//	  go test -v ./pkg/exchange/backpack/backpackapi/ -run TestWebSocketRecordPrivate
func TestWebSocketRecordPrivate(t *testing.T) {
	if os.Getenv("TEST_BACKPACK_WS_RECORD") == "" {
		t.Skip("TEST_BACKPACK_WS_RECORD is not set, skipping the live websocket recording")
	}

	key, secret, ok := testutil.IntegrationTestConfigured(t, "BACKPACK")
	if !ok {
		t.Skip("BACKPACK api key is not configured")
	}

	client := NewClient()
	if err := client.Auth(key, secret); err != nil {
		t.Fatalf("unable to authenticate: %v", err)
	}

	req, err := client.NewWebsocketSubscribeRequest([]string{
		"account.orderUpdate",
		"account.balanceUpdate",
		"account.positionUpdate",
	})

	if err != nil {
		t.Fatalf("unable to build the subscribe request: %v", err)
	}

	// the private streams only emit on account activity, so generate some while recording
	go func() {
		time.Sleep(3 * time.Second)

		ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
		defer cancel()

		if err := placeAndCancelProbeOrder(ctx, client); err != nil {
			logrus.Errorf("unable to generate account activity: %v", err)
		}
	}()

	recordWebSocketMessages(t, privateMessageRecordFile, 50*time.Second, []WebsocketRequest{req})
}

// placeAndCancelProbeOrder submits a post-only order away from the market and cancels it, so
// that the private streams have something to report.
func placeAndCancelProbeOrder(ctx context.Context, client *RestClient) error {
	const symbol = "USDT_USDC"

	ticker, err := client.NewGetTickerRequest().Symbol(symbol).Do(ctx)
	if err != nil {
		return err
	}

	// stay inside the mean mark price band of the market
	price := ticker.LastPrice.Mul(fixedpoint.NewFromFloat(1.008)).Round(4, fixedpoint.Down)

	order, err := client.NewPlaceOrderRequest().
		Symbol(symbol).
		Side(SideAsk).
		OrderType(OrderTypeLimit).
		TimeInForce(TimeInForceGTC).
		PostOnly(true).
		Price(price.String()).
		Quantity("20").
		Do(ctx)

	if err != nil {
		return err
	}

	logrus.Infof("probe order placed: %s", order.Id)

	time.Sleep(3 * time.Second)

	if _, err := client.NewCancelOrderRequest().Symbol(symbol).OrderId(order.Id).Do(ctx); err != nil {
		return err
	}

	logrus.Infof("probe order cancelled: %s", order.Id)
	return nil
}

// TestParseWebsocketMessagesFromFile replays the recorded websocket frames through the parser.
//
// This is the offline integration test: it runs in CI without credentials and asserts that
// every recorded frame decodes and that the decoded values are sane.
func TestParseWebsocketMessagesFromFile(t *testing.T) {
	for _, recordFile := range []string{publicMessageRecordFile, privateMessageRecordFile} {
		t.Run(recordFile, func(t *testing.T) {
			file, err := os.Open(recordFile)
			if err != nil {
				t.Fatalf("failed to open the record file: %v", err)
			}

			defer file.Close()

			counts := map[string]int{}
			var unhandled []string

			scanner := bufio.NewScanner(file)
			// the depth snapshots can exceed the default 64KB line limit
			scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

			lineNum := 0
			for scanner.Scan() {
				lineNum++

				line := scanner.Bytes()
				if len(line) == 0 {
					continue
				}

				event, err := ParseWebsocketMessage(line)
				if !assert.NoError(t, err, "parse error at line %d: %s", lineNum, string(line)) {
					continue
				}

				if event == nil {
					unhandled = append(unhandled, string(line))
					continue
				}

				counts[fmt.Sprintf("%T", event)]++
				assertWebsocketEvent(t, event, lineNum, line)
			}

			assert.NoError(t, scanner.Err())
			assert.NotEmpty(t, counts, "the record file should contain parsable events")

			t.Logf("parsed events: %+v", counts)
			if len(unhandled) > 0 {
				t.Logf("unhandled frames: %d, first: %s", len(unhandled), unhandled[0])
			}
		})
	}
}

// assertWebsocketEvent checks the invariants that must hold for every event of a given type.
func assertWebsocketEvent(t *testing.T, event interface{}, lineNum int, line []byte) {
	t.Helper()

	switch e := event.(type) {
	case *WebsocketError:
		t.Errorf("unexpected error frame at line %d: %v", lineNum, e)

	case *BookTickerEvent:
		assert.NotEmpty(t, e.Symbol, "line %d", lineNum)
		assert.True(t, e.BidPrice.Sign() > 0, "line %d: %s", lineNum, line)
		assert.True(t, e.AskPrice.Sign() > 0, "line %d: %s", lineNum, line)
		assert.True(t, e.BidPrice.Compare(e.AskPrice) < 0, "the bid must be below the ask, line %d", lineNum)
		assert.False(t, e.EventTime.Time().IsZero(), "line %d", lineNum)

	case *DepthEvent:
		assert.NotEmpty(t, e.Symbol, "line %d", lineNum)
		// an update carries at least one side
		assert.True(t, len(e.Asks) > 0 || len(e.Bids) > 0, "line %d: %s", lineNum, line)
		assert.NotZero(t, e.FirstUpdateId, "line %d", lineNum)
		assert.GreaterOrEqual(t, e.LastUpdateId, e.FirstUpdateId, "line %d", lineNum)
		for _, level := range append(append([]PriceLevel{}, e.Asks...), e.Bids...) {
			assert.True(t, level.Price.Sign() > 0, "line %d: %s", lineNum, line)
		}

	case *TradeEvent:
		assert.NotEmpty(t, e.Symbol, "line %d", lineNum)
		assert.NotZero(t, e.TradeId, "line %d", lineNum)
		assert.True(t, e.Price.Sign() > 0, "line %d: %s", lineNum, line)
		assert.True(t, e.Quantity.Sign() > 0, "line %d: %s", lineNum, line)

	case *TickerEvent:
		assert.NotEmpty(t, e.Symbol, "line %d", lineNum)
		assert.True(t, e.Close.Sign() > 0, "line %d: %s", lineNum, line)
		assert.True(t, e.High.Compare(e.Low) >= 0, "line %d", lineNum)

	case *KLineEvent:
		assert.NotEmpty(t, e.Symbol, "line %d", lineNum)
		// the interval is not in the payload, it has to come from the stream name
		assert.NotEmpty(t, e.Interval, "the kline interval should be taken from the stream name, line %d", lineNum)
		assert.False(t, e.StartTime.Time().IsZero(), "line %d: %s", lineNum, line)
		assert.True(t, e.EndTime.Time().After(e.StartTime.Time()), "line %d: %s", lineNum, line)

	case *MarkPriceEvent:
		assert.NotEmpty(t, e.Symbol, "line %d", lineNum)
		assert.True(t, e.MarkPrice.Sign() > 0, "line %d: %s", lineNum, line)

	case *BalanceUpdateEvent:
		assert.NotEmpty(t, e.Asset, "line %d", lineNum)
		assert.False(t, e.Available.Sign() < 0, "line %d: %s", lineNum, line)
		assert.False(t, e.Locked.Sign() < 0, "line %d: %s", lineNum, line)

	case *OrderUpdateEvent:
		assert.NotEmpty(t, e.Symbol, "line %d", lineNum)
		assert.NotEmpty(t, e.OrderId, "line %d", lineNum)
		assert.NotEmpty(t, e.Side, "line %d", lineNum)
		assert.NotEmpty(t, e.Status, "line %d", lineNum)
		// the websocket spells the order type upper case, it must normalize to the REST value
		assert.Contains(t, []OrderType{OrderTypeLimit, OrderTypeMarket}, e.OrderType.OrderType(),
			"line %d: %s", lineNum, line)

	default:
		t.Errorf("unexpected event type %T at line %d", event, lineNum)
	}
}
