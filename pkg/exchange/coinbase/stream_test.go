package coinbase

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/stretchr/testify/assert"
)

func TestStreamSubCmdString(t *testing.T) {
	var subCmd interface{}
	subCmd = subscribeMsgType1{
		Type: "subscribe",
		Channels: []channelType{
			{
				Name:       "ticker",
				ProductIDs: []string{"BTC-USD"},
			},
			{
				Name:       "matches",
				ProductIDs: []string{"BTC-USD"},
			},
		},
		authMsg: authMsg{
			Signature:  "signature",
			Key:        "<secret_key!>",
			Passphrase: "<secret_passphrase!>",
			Timestamp:  "timestamp",
		},
	}
	outStr := fmt.Sprintf("%s", subCmd)
	assert.False(t, strings.Contains(outStr, "<secret_key!>"))
	assert.False(t, strings.Contains(outStr, "<secret_passphrase!>"))

	subCmd = subscribeMsgType2{
		Type:       "subscribe",
		Channels:   []types.Channel{},
		ProductIDs: []string{},
		AccountIDs: []string{},
		authMsg: authMsg{
			Signature:  "signature",
			Key:        "<secret_key!>",
			Passphrase: "<secret_passphrase!>",
			Timestamp:  "timestamp",
		},
	}
	outStr = fmt.Sprintf("%s", subCmd)
	assert.False(t, strings.Contains(outStr, "<secret_key!>"))
	assert.False(t, strings.Contains(outStr, "<secret_passphrase!>"))
}

func TestStreamBasic(t *testing.T) {
	productIDs := []string{"BTCUSD", "ETHUSD"}

	t.Run("TestStatus", func(t *testing.T) {
		stream, isRecording, saveRecord := getTestStreamOrSkip(t)
		stream.SetPublicOnly()
		chanStatus := make(chan StatusMessage)
		stream.Subscribe(statusChannel, "", types.SubscribeOptions{})

		if isRecording {
			err := stream.Connect(context.Background())
			assert.NoError(t, err)
			time.Sleep(10 * time.Second)
			saveRecord()
			err = stream.Close()
			assert.NoError(t, err)
			return
		}
		stream.OnStatusMessage(func(m *StatusMessage) {
			assert.NotNil(t, m)
			// t.Log("get status message")
			chanStatus <- *m
		})
		replayStreamRecord(t, stream, nil)
		select {
		case msg := <-chanStatus:
			assert.IsType(t, StatusMessage{}, msg)
		case <-time.After(time.Second * 10):
			t.Fatal("No message from status channel after 10 seconds")
		}
	})

	t.Run("TestTicker", func(t *testing.T) {
		stream, isRecording, saveRecord := getTestStreamOrSkip(t)
		stream.SetPublicOnly()
		for _, productID := range productIDs {
			stream.Subscribe(tickerChannel, productID, types.SubscribeOptions{})
		}

		if isRecording {
			err := stream.Connect(context.Background())
			assert.NoError(t, err)
			time.Sleep(5 * time.Second)
			saveRecord()
			err = stream.Close()
			assert.NoError(t, err)
			return
		}

		chanTicker := make(chan TickerMessage)
		stream.OnTickerMessage(func(m *TickerMessage) {
			assert.NotNil(t, m)
			// t.Logf("get ticker message: %v", *m)
			chanTicker <- *m
		})
		replayStreamRecord(t, stream, nil)

		select {
		case msg := <-chanTicker:
			assert.IsType(t, TickerMessage{}, msg)
		case <-time.After(time.Second * 10):
			t.Fatal("No message from ticker channel after 10 seconds")
		}
	})

	t.Run("TestMatch", func(t *testing.T) {
		stream, isRecording, saveRecord := getTestStreamOrSkip(t)
		stream.SetPublicOnly()
		for _, productID := range productIDs {
			stream.Subscribe(matchesChannel, productID, types.SubscribeOptions{})
		}

		if isRecording {
			err := stream.Connect(context.Background())
			assert.NoError(t, err)
			time.Sleep(5 * time.Second)
			saveRecord()
			err = stream.Close()
			assert.NoError(t, err)
			return
		}

		chanMatch := make(chan MatchMessage)
		stream.OnMatchMessage(func(m *MatchMessage) {
			assert.NotNil(t, m)
			// t.Logf("get match message: %v", *m)
			chanMatch <- *m
		})
		replayStreamRecord(t, stream, nil)
		// wait for 5 seconds to receive match message
		time.Sleep(time.Second * 5)
		select {
		case msg := <-chanMatch:
			assert.IsType(t, MatchMessage{}, msg)
		case <-time.After(time.Second * 10):
			t.Fatal("No message from match channel after 10 seconds")
		}
	})

	// TODO: test Rfq message
}

func TestStreamFull(t *testing.T) {
	productIDs := []string{"BTCUSD", "ETHUSD"}
	stream, isRecording, saveRecord := getTestStreamOrSkip(t)
	stream.SetPublicOnly()
	for _, productID := range productIDs {
		stream.Subscribe(fullChannel, productID, types.SubscribeOptions{})
	}

	if isRecording {
		err := stream.Connect(context.Background())
		assert.NoError(t, err)
		time.Sleep(10 * time.Second)
		saveRecord()
		err = stream.Close()
		assert.NoError(t, err)
		return
	}

	// received -> open* -> change* -> match? -> done
	c := make(chan struct{}, 10)
	stream.OnReceivedMessage(func(m *ReceivedMessage) {
		// t.Log("get received message")
		assert.NotNil(t, m)
		c <- struct{}{}
	})
	stream.OnOpenMessage(func(m *OpenMessage) {
		// t.Log("get open message")
		assert.NotNil(t, m)
		c <- struct{}{}
	})
	stream.OnDoneMessage(func(m *DoneMessage) {
		// t.Log("get done message")
		assert.NotNil(t, m)
		c <- struct{}{}
	})
	stream.OnMatchMessage(func(m *MatchMessage) {
		// t.Log("get match message")
		assert.NotNil(t, m)
		c <- struct{}{}
	})
	stream.OnChangeMessage(func(m *ChangeMessage) {
		// t.Log("get change message")
		assert.NotNil(t, m)
		c <- struct{}{}
	})
	stream.OnActivateMessage(func(m *ActivateMessage) {
		// t.Log("get activate message")
		assert.NotNil(t, m)
		c <- struct{}{}
	})
	replayStreamRecord(t, stream, nil)

	select {
	case <-c:
	case <-time.After(time.Second * 10):
		t.Fatal("No message from full channel after 10 seconds")
	}
}

func TestStreamLevel2(t *testing.T) {
	stream, isRecording, saveRecord := getTestStreamOrSkip(t)
	stream.SetPublicOnly()
	productIDs := []string{"BTCUSD"}
	for _, productID := range productIDs {
		stream.Subscribe(level2Channel, productID, types.SubscribeOptions{})
	}

	if isRecording {
		err := stream.Connect(context.Background())
		assert.NoError(t, err)
		time.Sleep(5 * time.Second)
		saveRecord()
		err = stream.Close()
		assert.NoError(t, err)
		return
	}

	c := make(chan string, 2)
	getSnapshot := false
	getUpdate := false
	stream.OnOrderbookSnapshotMessage(func(m *OrderBookSnapshotMessage) {
		// t.Log("get orderbook snapshot message")
		assert.NotNil(t, m)
		c <- "snapshot"
	})
	stream.OnOrderbookUpdateMessage(func(m *OrderBookUpdateMessage) {
		// t.Log("get orderbook update message")
		assert.NotNil(t, m)
		c <- "update"
	})
	replayStreamRecord(t, stream, nil)

	for {
		select {
		case msg := <-c:
			switch msg {
			case "snapshot":
				getSnapshot = true
			case "update":
				getUpdate = true
			}
		case <-time.After(time.Second * 10):
			t.Fatal("No message from level2 channel after 10 seconds")
		default:
			// do nothing
		}
		if getSnapshot && getUpdate {
			break
		}
	}
}

func TestStreamBalance(t *testing.T) {

	accountsStr := os.Getenv("COINBASE_ACCOUNT_IDS")
	accounts := strings.Split(accountsStr, ",")

	stream, isRecording, saveRecord := getTestStreamOrSkip(t)
	stream.SetPublicOnly()

	if len(accounts) == 0 && isRecording {
		t.Fatal("No account IDs found")
	}

	for _, accountID := range accounts {
		stream.Subscribe(balanceChannel, accountID, types.SubscribeOptions{})
	}

	if isRecording {
		err := stream.Connect(context.Background())
		assert.NoError(t, err)
		time.Sleep(10 * time.Second)
		saveRecord()
		err = stream.Close()
		assert.NoError(t, err)
		return
	}

	c := make(chan struct{}, 1)
	stream.OnBalanceMessage(func(m *BalanceMessage) {
		assert.NotNil(t, m)
		// t.Logf("get balance message: %v", *m)
		c <- struct{}{}
	})
	replayStreamRecord(t, stream, nil)
	select {
	case <-c:
	case <-time.After(time.Second * 10):
		t.Fatal("No message from balance channel after 10 seconds")
	}
}

func TestStreamBbgoChannels(t *testing.T) {
	t.Run("TestBook", func(t *testing.T) {
		stream, isRecording, saveRecord := getTestStreamOrSkip(t)
		stream.SetPublicOnly()
		stream.Subscribe(types.BookChannel, "BTCUSD", types.SubscribeOptions{})

		if isRecording {
			err := stream.Connect(context.Background())
			assert.NoError(t, err)
			time.Sleep(5 * time.Second)
			saveRecord()
			err = stream.Close()
			assert.NoError(t, err)
			return
		}

		c := make(chan string, 1)
		stream.OnOrderbookSnapshotMessage(func(m *OrderBookSnapshotMessage) {
			assert.NotNil(t, m)
			c <- "snapshot"
		})
		stream.OnOrderbookUpdateMessage(func(m *OrderBookUpdateMessage) {
			assert.NotNil(t, m)
			c <- "update"
		})
		replayStreamRecord(t, stream, nil)
		getSnapshot := false
		getUpdate := false
	outer:
		for {
			select {
			case sig := <-c:
				switch sig {
				case "snapshot":
					getSnapshot = true
				case "update":
					getUpdate = true
				}
				if getSnapshot && getUpdate {
					break outer
				}
			case <-time.After(time.Second * 10):
				t.Fatal("No message from book channel after 10 seconds")
			}
		}
	})

	t.Run("TestMarketTrade", func(t *testing.T) {
		stream, isRecording, saveRecord := getTestStreamOrSkip(t)
		stream.SetPublicOnly()
		stream.Subscribe(types.MarketTradeChannel, "BTCUSD", types.SubscribeOptions{})

		if isRecording {
			err := stream.Connect(context.Background())
			assert.NoError(t, err)
			time.Sleep(5 * time.Second)
			saveRecord()
			err = stream.Close()
			assert.NoError(t, err)
			return
		}

		c := make(chan struct{}, 1)
		stream.OnMarketTrade(func(m types.Trade) {
			c <- struct{}{}
		})
		replayStreamRecord(t, stream, nil)
		select {
		case <-c:
		case <-time.After(time.Second * 10):
			t.Fatal("No message from market trade channel after 10 seconds")
		}
	})
}

func TestStreamInvalidCredentials(t *testing.T) {
	key := ""
	secret := ""
	passphrase := ""
	exchange := New(key, secret, passphrase, 0)
	stream := exchange.NewStream()
	// should error
	err := stream.Connect(context.Background())
	assert.Error(t, err)
}

func TestPrivateChannelSymbols(t *testing.T) {
	exchange := New("", "", "", 0)
	stream := NewStream(exchange, "", "", "")
	stream.SetPrivateChannelSymbols([]string{"BTCUSD", "ETHUSD"})
	assert.Equal(t, []string{"BTC-USD", "ETH-USD"}, stream.privateChannelLocalSymbols())
}

// TestStreamUserTrades run test on collecting trades after a submission
func TestStreamUserTrades(t *testing.T) {
	// create a user data stream
	symbol := "ETHUSDT"
	stream, isRecording, saveRecord := getTestStreamOrSkip(t)
	stream.SetPrivateChannelSymbols([]string{symbol})
	if isRecording {
		err := stream.Connect(context.Background())
		assert.NoError(t, err)
		markets, err := stream.exchange.QueryMarkets(
			context.Background(),
		)
		assert.NoError(t, err)
		market, ok := markets[symbol]
		assert.True(t, ok, "market %s not found", symbol)
		createdOrder, err := stream.exchange.SubmitOrder(
			context.Background(),
			types.SubmitOrder{
				Market:   market,
				Symbol:   symbol,
				Type:     types.OrderTypeMarket,
				Side:     types.SideTypeSell,
				Quantity: fixedpoint.MustNewFromString("0.001"),
			},
		)
		t.Logf("created order: %+v", createdOrder)
		assert.NoError(t, err)
		time.Sleep(30 * time.Second)
		saveRecord()
		err = stream.Close()
		assert.NoError(t, err)
		return
	}
	var tradeSides []types.SideType
	stream.OnTradeUpdate(
		func(trade types.Trade) {
			tradeSides = append(tradeSides, trade.Side)
		},
	)
	doneC := make(chan struct{})
	replayStreamRecord(t, stream, doneC)
	<-doneC
	for _, side := range tradeSides {
		assert.Equal(t, side, types.SideTypeSell)
	}
}

func getTestStreamOrSkip(t *testing.T) (*Stream, bool, func()) {
	key, secret, passphrase, ok := IntegrationTestWithPassphraseConfigured(t, "COINBASE")
	isRecording := os.Getenv("TEST_COINBASE_STREAM_RECORD") == "1"
	if isRecording && !ok {
		t.Fatal("COINBASE_* env vars not set")
	}
	exchange := New(key, secret, passphrase, 0)
	stream := NewStream(exchange, key, secret, passphrase)
	saveRecord := func() {}
	if isRecording {
		// unset parser to capture the raw messages
		stream.SetParser(nil)
		filePath := fmt.Sprintf("testdata/%s.json", t.Name())
		if err := os.MkdirAll(path.Dir(filePath), 0755); err != nil && !os.IsNotExist(err) {
			t.Fatalf("fail to create directory: %s", err.Error())
		}

		file, err := os.Create(filePath)
		if err != nil {
			t.Fatalf("fail to create testdata file: %s", err.Error())
		}
		entities := []interface{}{}
		stream.OnRawMessage(func(msg []byte) {
			entity, err := parseMessage(msg)
			if err != nil {
				t.Fatalf("fail to parse message: %s", string(msg))
			}
			entities = append(entities, entity)
		})
		saveRecord = func() {
			defer file.Close()
			if len(entities) == 0 {
				return
			}
			encoder := json.NewEncoder(file)
			encoder.SetIndent("", "  ")
			if err := encoder.Encode(entities); err != nil {
				t.Fatalf("fail to encode testdata file: %s", err.Error())
			}
			t.Logf("recorded %d messages to %s", len(entities), file.Name())
		}
	}
	return stream, isRecording, saveRecord
}

func replayStreamRecord(t *testing.T, stream *Stream, doneC chan struct{}) {
	filePath := fmt.Sprintf("testdata/%s.json", t.Name())
	file, err := os.Open(filePath)
	if err != nil {
		t.Fatalf("fail to open testdata file: %s", err.Error())
	}
	defer file.Close()

	var entities []interface{}
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&entities); err != nil {
		t.Fatalf("fail to decode testdata file: %s", err.Error())
	}
	go func() {
		for _, entity := range entities {
			data, err := json.Marshal(entity)
			if err != nil {
				t.Logf("fail to marshal entity: %s", err.Error())
				return
			}
			event, err := parseMessage(data)
			if err != nil {
				t.Logf("fail to parse message: %s", err.Error())
				return
			}
			stream.dispatchEvent(event)
		}
		if doneC != nil {
			doneC <- struct{}{}
		}
	}()
}
