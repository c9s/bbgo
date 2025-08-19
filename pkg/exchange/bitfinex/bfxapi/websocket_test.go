package bfxapi

import (
	"bufio"
	"context"
	"os"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/testutil"
)

const publicMessageRecordFile = "testdata/bitfinex_ws_raw.jsonl"

func TestWebSocketRecord(t *testing.T) {
	if os.Getenv("TEST_BFX_WS_RECORD") == "" {
		t.Skip("TEST_BFX_WS_RECORD env not set, skipping real websocket test")
	}

	url := PublicWebSocketURL
	c, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("failed to connect to bitfinex ws: %v", err)
	}
	defer c.Close()

	file, err := os.Create(publicMessageRecordFile)
	if err != nil {
		t.Fatalf("failed to create output file: %v", err)
	}

	defer func() {
		if err := file.Sync(); err != nil {
			logrus.Errorf("failed to sync file: %v", err)
		}
		if err := file.Close(); err != nil {
			logrus.Errorf("failed to close file: %v", err)
		}
	}()

	// use WebSocketRequest struct for subscription
	subscribe := func(channel Channel, symbol string) error {
		req := WebSocketRequest{
			Event:   "subscribe",
			Channel: channel,
			Symbol:  symbol,
		}
		return c.WriteJSON(req)
	}

	if err := subscribe(ChannelTrades, "tBTCUSD"); err != nil {
		t.Fatalf("failed to subscribe trades: %v", err)
	}
	if err := subscribe(ChannelBook, "tBTCUSD"); err != nil {
		t.Fatalf("failed to subscribe book: %v", err)
	}
	if err := subscribe(ChannelBook, "fUST"); err != nil {
		t.Fatalf("failed to subscribe book: %v", err)
	}
	if err := subscribe(ChannelTicker, "tBTCUSD"); err != nil {
		t.Fatalf("failed to subscribe ticker: %v", err)
	}
	if err := subscribe(ChannelTicker, "fUST"); err != nil {
		t.Fatalf("failed to subscribe ticker: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			t.Log("timeout reached, closing connection")
			return
		default:
			if err := c.SetReadDeadline(time.Now().Add(10 * time.Second)); err != nil {
				logrus.Errorf("failed to set read deadline: %v", err)
				return
			}
			_, msg, err := c.ReadMessage()
			if err != nil {
				if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					t.Log("websocket closed normally")
					return
				}
				logrus.Errorf("read error: %v", err)
				return
			}
			if _, err := file.Write(append(msg, '\n')); err != nil {
				logrus.Errorf("failed to write message: %v", err)
			}
			t.Logf("received message: %s", string(msg))
		}
	}
}

func TestParserParseFromFile(t *testing.T) {
	file, err := os.Open(publicMessageRecordFile)
	if err != nil {
		t.Fatalf("failed to open record file: %v", err)
	}
	defer file.Close()

	parser := NewParser()

	scanner := bufio.NewScanner(file)
	lineNum := 0
	errorCount := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		result, err := parser.Parse(line)
		if err != nil {
			errorCount++
			assert.NoError(t, err, "parse error at line %d: %v", lineNum, err)
			t.Logf("skipping line %d due to parse error: %s", lineNum, string(line))
			continue
		}

		// t.Logf("parsed line %d: %T %+v", lineNum, result, result)

		switch tr := result.(type) { // tr = typed result
		case *WebSocketResponse:
			assert.NotEmpty(t, tr.Event)
			switch tr.Event {
			case "subscribed":
				assert.NotZero(t, tr.ChanId)
			}
		case *TickerEvent:
			assert.NotZero(t, tr.ChannelID)
			assert.False(t, tr.Ask.IsZero())
			assert.False(t, tr.Bid.IsZero())
		case *FundingTickerEvent:
			assert.NotZero(t, tr.ChannelID)
			assert.False(t, tr.FRR.IsZero())
		case *MarketTradeEvent:
			assert.NotZero(t, tr.ChannelID)
			assert.NotZero(t, tr.ID)
			assert.True(t, !tr.Price.IsZero())
		case []MarketTradeEvent: // trade snapshot
			for _, trade := range tr {
				assert.NotZero(t, trade.ChannelID)
				assert.NotZero(t, trade.ID)
				assert.False(t, trade.Price.IsZero(), "price should not be zero for market trades")
				assert.False(t, trade.Amount.IsZero(), "amount should not be zero for market trades")
			}

		case *FundingMarketTradeEvent:
			assert.NotZero(t, tr.ChannelID)
			assert.NotZero(t, tr.ID)
			assert.False(t, tr.Amount.IsZero(), "amount should not be zero for funding trades")
		case *FundingBookEvent:
			assert.NotZero(t, tr.ChannelID)
			assert.NotZero(t, tr.Period)
			assert.False(t, tr.Amount.IsZero(), "amount should not be zero for funding book events")
			assert.False(t, tr.Rate.IsZero(), "rate should not be zero for funding book events")
		case []FundingBookEvent:
			for _, fundingBookEvent := range tr {
				assert.NotZero(t, fundingBookEvent.ChannelID)
				assert.NotZero(t, fundingBookEvent.Period)
				assert.False(t, fundingBookEvent.Amount.IsZero(), "amount should not be zero for funding book events")
				assert.False(t, fundingBookEvent.Rate.IsZero(), "rate should not be zero for funding book events")
			}
		case *BookEvent:
			assert.NotZero(t, tr.ChannelID)
			assert.False(t, tr.Price.IsZero(), "price should not be zero for book events")
			assert.False(t, tr.Amount.IsZero(), "amount should not be zero for book events")
		case []BookEvent:
			for _, bookEvent := range tr {
				assert.NotZero(t, bookEvent.ChannelID)
				assert.False(t, bookEvent.Price.IsZero(), "price should not be zero for book events")
				assert.False(t, bookEvent.Amount.IsZero(), "amount should not be zero for book events")
			}
		case *HeartBeatEvent:
		default:
			t.Logf("unhandled type at line %d: %T %+v", lineNum, result, result)
		}
	}

	if err := scanner.Err(); err != nil {
		t.Fatalf("scanner error: %v", err)
	}

	t.Logf("total parse errors: %d", errorCount)
}

func TestWebSocketRecordPrivate(t *testing.T) {
	if false && os.Getenv("TEST_BFX_WS_PRIVATE_RECORD") == "" {
		t.Skip("TEST_BFX_WS_PRIVATE_RECORD env not set, skipping real websocket test")
	}

	key, secret, ok := testutil.IntegrationTestConfigured(t, "BITFINEX")
	if !ok {
		t.Skip("BITFINEX integration test not configured, skipping private websocket test")
	}

	url := PrivateWebSocketURL
	c, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("failed to connect to bitfinex private ws: %v", err)
	}
	defer c.Close()

	file, err := os.Create("testdata/bitfinex_ws_private_raw.jsonl")
	if err != nil {
		t.Fatalf("failed to create output file: %v", err)
	}
	defer func() {
		if err := file.Sync(); err != nil {
			logrus.Errorf("failed to sync file: %v", err)
		}
		if err := file.Close(); err != nil {
			logrus.Errorf("failed to close file: %v", err)
		}
	}()

	// send auth message
	authMsg := GenerateAuthRequest(key, secret)
	if err := c.WriteJSON(authMsg); err != nil {
		t.Fatalf("failed to send auth message: %v", err)
	}

	// subscribe to wallet channel (see Bitfinex docs)
	walletSub := WebSocketRequest{
		Event:   "subscribe",
		Channel: "wallet",
	}
	if err := c.WriteJSON(walletSub); err != nil {
		t.Fatalf("failed to subscribe wallet: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			t.Log("timeout reached, closing connection")
			return
		default:
			if err := c.SetReadDeadline(time.Now().Add(10 * time.Second)); err != nil {
				logrus.Errorf("failed to set read deadline: %v", err)
				return
			}
			_, msg, err := c.ReadMessage()
			if err != nil {
				if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					t.Log("websocket closed normally")
					return
				}
				logrus.Errorf("read error: %v", err)
				return
			}
			if _, err := file.Write(append(msg, '\n')); err != nil {
				logrus.Errorf("failed to write message: %v", err)
			}
			t.Logf("received message: %s", string(msg))
		}
	}
}
