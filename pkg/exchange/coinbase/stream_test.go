package coinbase

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/c9s/bbgo/pkg/types"
	"github.com/stretchr/testify/assert"
)

func Test_SubCmdString(t *testing.T) {
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

	t.Run("Test Status", func(t *testing.T) {
		stream := getTestStreamOrSkip(t)
		stream.SetPublicOnly()
		chanStatus := make(chan StatusMessage)

		stream.Subscribe(statusChannel, "", types.SubscribeOptions{})
		stream.OnStatusMessage(func(m *StatusMessage) {
			assert.NotNil(t, m)
			// t.Log("get status message")
			chanStatus <- *m
		})

		err := stream.Connect(context.Background())
		assert.NoError(t, err)
		select {
		case msg := <-chanStatus:
			assert.IsType(t, StatusMessage{}, msg)
		case <-time.After(time.Second * 10):
			t.Fatal("No message from status channel after 10 seconds")
		}
		err = stream.Close()
		assert.NoError(t, err)
	})

	t.Run("Test Ticker", func(t *testing.T) {
		stream := getTestStreamOrSkip(t)
		stream.SetPublicOnly()
		chanTicker := make(chan TickerMessage)

		for _, productID := range productIDs {
			stream.Subscribe(tickerChannel, productID, types.SubscribeOptions{})
		}
		stream.OnTickerMessage(func(m *TickerMessage) {
			assert.NotNil(t, m)
			// t.Logf("get ticker message: %v", *m)
			chanTicker <- *m
		})

		err := stream.Connect(context.Background())
		assert.NoError(t, err)
		select {
		case msg := <-chanTicker:
			assert.IsType(t, TickerMessage{}, msg)
		case <-time.After(time.Second * 10):
			t.Fatal("No message from ticker channel after 10 seconds")
		}
		err = stream.Close()
		assert.NoError(t, err)
	})

	t.Run("Test Match", func(t *testing.T) {
		stream := getTestStreamOrSkip(t)
		stream.SetPublicOnly()
		chanMatch := make(chan MatchMessage)

		for _, productID := range productIDs {
			stream.Subscribe(matchesChannel, productID, types.SubscribeOptions{})
		}
		stream.OnMatchMessage(func(m *MatchMessage) {
			assert.NotNil(t, m)
			// t.Logf("get match message: %v", *m)
			chanMatch <- *m
		})

		err := stream.Connect(context.Background())
		assert.NoError(t, err)
		// wait for 5 seconds to receive match message
		time.Sleep(time.Second * 5)
		select {
		case msg := <-chanMatch:
			assert.IsType(t, MatchMessage{}, msg)
		case <-time.After(time.Second * 10):
			t.Fatal("No message from match channel after 10 seconds")
		}
		err = stream.Close()
		assert.NoError(t, err)
	})

	// TODO: test Rfq message
}

func TestStreamFull(t *testing.T) {
	t.Run("Run Full", func(t *testing.T) {
		productIDs := []string{"BTCUSD", "ETHUSD"}
		c := make(chan struct{}, 10)
		stream := getTestStreamOrSkip(t)
		stream.SetPublicOnly()
		for _, productID := range productIDs {
			stream.Subscribe(fullChannel, productID, types.SubscribeOptions{})
		}

		// received -> open* -> change* -> match? -> done
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

		err := stream.Connect(context.Background())
		assert.NoError(t, err)
		select {
		case <-c:
		case <-time.After(time.Second * 10):
			t.Fatal("No message from full channel after 10 seconds")
		}
		err = stream.Close()
		assert.NoError(t, err)
	})

}

func TestLevel2(t *testing.T) {
	t.Run("Run Level2", func(t *testing.T) {
		stream := getTestStreamOrSkip(t)
		stream.SetPublicOnly()
		c := make(chan string, 2)
		productIDs := []string{"BTCUSD"}
		getSnapshot := false
		getUpdate := false

		for _, productID := range productIDs {
			stream.Subscribe(level2Channel, productID, types.SubscribeOptions{})
		}

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

		err := stream.Connect(context.Background())
		assert.NoError(t, err)
		for {
			select {
			case msg := <-c:
				if msg == "snapshot" {
					getSnapshot = true
				} else if msg == "update" {
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
		err = stream.Close()
		assert.NoError(t, err)
	})
}

func TestBalance(t *testing.T) {

	t.Run("Run Balance", func(t *testing.T) {
		accountsStr := os.Getenv("COINBASE_ACCOUNT_IDS")
		accounts := strings.Split(accountsStr, ",")

		c := make(chan struct{}, 1)
		stream := getTestStreamOrSkip(t)
		stream.SetPublicOnly()
		for _, accountID := range accounts {
			stream.Subscribe(balanceChannel, accountID, types.SubscribeOptions{})
		}
		stream.OnBalanceMessage(func(m *BalanceMessage) {
			assert.NotNil(t, m)
			// t.Logf("get balance message: %v", *m)
			c <- struct{}{}
		})
		err := stream.Connect(context.Background())
		assert.NoError(t, err)
		select {
		case <-c:
		case <-time.After(time.Second * 10):
			t.Fatal("No message from balance channel after 10 seconds")
		}
		err = stream.Close()
		assert.NoError(t, err)
	})
}

func TestStreamBbgoChannels(t *testing.T) {
	t.Run("Test Book", func(t *testing.T) {
		c := make(chan string, 1)
		stream := getTestStreamOrSkip(t)
		stream.SetPublicOnly()
		stream.Subscribe(types.BookChannel, "BTCUSD", types.SubscribeOptions{})
		stream.OnOrderbookSnapshotMessage(func(m *OrderBookSnapshotMessage) {
			assert.NotNil(t, m)
			c <- "snapshot"
		})
		stream.OnOrderbookUpdateMessage(func(m *OrderBookUpdateMessage) {
			assert.NotNil(t, m)
			c <- "update"
		})
		err := stream.Connect(context.Background())
		assert.NoError(t, err)
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
		err = stream.Close()
		assert.NoError(t, err)
	})

	t.Run("Test Market Trade", func(t *testing.T) {
		c := make(chan struct{}, 1)
		stream := getTestStreamOrSkip(t)
		stream.SetPublicOnly()
		stream.Subscribe(types.MarketTradeChannel, "BTCUSD", types.SubscribeOptions{})
		stream.OnMarketTrade(func(m types.Trade) {
			c <- struct{}{}
		})
		err := stream.Connect(context.Background())
		assert.NoError(t, err)
		select {
		case <-c:
		case <-time.After(time.Second * 10):
			t.Fatal("No message from market trade channel after 10 seconds")
		}
		err = stream.Close()
		assert.NoError(t, err)
	})
}

func TestStreamInvalidCredentials(t *testing.T) {
	t.Run("Test Book Channel Panic", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				return
			}
			t.Fatal("Expected panic but got none")
		}()
		key := ""
		secret := ""
		passphrase := ""
		exchange := New(key, secret, passphrase, 0)
		stream := exchange.NewStream()
		stream.Subscribe(types.BookChannel, "BTCUSD", types.SubscribeOptions{})
		// should panic
		_ = stream.Connect(context.Background())
	})

	t.Run("Test Market Trade Channel Panic", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				return
			}
			t.Fatal("Expected panic but got none")
		}()
		key := ""
		secret := ""
		passphrase := ""
		exchange := New(key, secret, passphrase, 0)
		stream := exchange.NewStream()
		stream.Subscribe(types.MarketTradeChannel, "BTCUSD", types.SubscribeOptions{})
		// should panic
		_ = stream.Connect(context.Background())
	})
}

func TestPrivateChannelSymbols(t *testing.T) {
	stream := getTestStreamOrSkip(t)
	stream.SetPrivateChannelSymbols([]string{"BTCUSD", "ETHUSD"})
	assert.Equal(t, []string{"BTC-USD", "ETH-USD"}, stream.privateChannelLocalSymbols())
}

func getTestStreamOrSkip(t *testing.T) *Stream {
	if isCI, _ := strconv.ParseBool(os.Getenv("CI")); isCI {
		t.Skip("skip test for CI")
	}

	key, secret, passphrase, ok := IntegrationTestWithPassphraseConfigured(t, "COINBASE")
	if !ok {
		t.Skip("COINBASE_* env vars not set")
	}
	exchange := New(key, secret, passphrase, 0)
	stream := NewStream(exchange, key, secret, passphrase)
	return stream
}
