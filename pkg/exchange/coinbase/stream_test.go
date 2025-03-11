package coinbase

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/c9s/bbgo/pkg/testutil"
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
		Channels:   []string{},
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
	c := make(chan any)
	productIDs := []string{"BTC-USD", "ETH-USD"}
	var msg any

	t.Run("Test Status", func(t *testing.T) {
		stream := getTestStreamOrSkip(t)
		stream.Subscribe("status", "", types.SubscribeOptions{})
		triggered := false
		stream.OnStatusMessage(func(m *StatusMessage) {
			if triggered {
				return
			}
			triggered = true
			assert.NotNil(t, m)
			// t.Log("get status message")
			c <- *m
		})
		err := stream.Connect(context.Background())
		assert.NoError(t, err)
	})
	msg = <-c
	assert.IsType(t, StatusMessage{}, msg)

	t.Run("Test Ticker", func(t *testing.T) {
		stream := getTestStreamOrSkip(t)
		for _, productID := range productIDs {
			stream.Subscribe("ticker", productID, types.SubscribeOptions{})
		}
		triggered := false
		stream.OnTickerMessage(func(m *TickerMessage) {
			if triggered {
				return
			}
			triggered = true
			assert.NotNil(t, m)
			// t.Logf("get ticker message: %v", *m)
			c <- *m
		})
		err := stream.Connect(context.Background())
		assert.NoError(t, err)
	})
	msg = <-c
	assert.IsType(t, TickerMessage{}, msg)

	t.Run("Test Match", func(t *testing.T) {
		stream := getTestStreamOrSkip(t)
		for _, productID := range productIDs {
			stream.Subscribe("matches", productID, types.SubscribeOptions{})
		}
		triggered := false
		stream.OnMatchMessage(func(m *MatchMessage) {
			if triggered {
				return
			}
			triggered = true
			assert.NotNil(t, m)
			// t.Logf("get match message: %v", *m)
			c <- *m
		})
		err := stream.Connect(context.Background())
		assert.NoError(t, err)
	})
	msg = <-c
	assert.IsType(t, MatchMessage{}, msg)

	// TODO: test Rfq message
}

func TestStreamFull(t *testing.T) {
	c := make(chan bool)
	productIDs := []string{"BTC-USD", "ETH-USD"}

	t.Run("Run Full", func(t *testing.T) {
		stream := getTestStreamOrSkip(t)
		for _, productID := range productIDs {
			stream.Subscribe("full", productID, types.SubscribeOptions{})
		}
		// TODO: test full order life cycle
		// received -> open+ -> change* -> match? -> done
		stream.OnReceivedMessage(func(m *ReceivedMessage) {
			// t.Log("get received message")
			assert.NotNil(t, m)
			c <- true
		})
		stream.OnOpenMessage(func(m *OpenMessage) {
			// t.Log("get open message")
			assert.NotNil(t, m)
			c <- true
		})
		stream.OnDoneMessage(func(m *DoneMessage) {
			// t.Log("get done message")
			assert.NotNil(t, m)
			c <- true
		})
		stream.OnMatchMessage(func(m *MatchMessage) {
			// t.Log("get match message")
			assert.NotNil(t, m)
			c <- true
		})
		stream.OnChangeMessage(func(m *ChangeMessage) {
			// t.Log("get change message")
			assert.NotNil(t, m)
			c <- true
		})
		stream.OnActivateMessage(func(m *ActivateMessage) {
			// t.Log("get activate message")
			assert.NotNil(t, m)
			c <- true
		})
		err := stream.Connect(context.Background())
		assert.NoError(t, err)
	})
	<-c
}

func TestLevel2(t *testing.T) {
	c := make(chan string)
	productIDs := []string{"BTC-USD"}

	getSnapshot := false
	getUpdate := false
	t.Run("Run Level2", func(t *testing.T) {
		stream := getTestStreamOrSkip(t)
		for _, productID := range productIDs {
			stream.Subscribe("level2", productID, types.SubscribeOptions{})
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
	})
	for {
		select {
		case msg := <-c:
			if msg == "snapshot" {
				getSnapshot = true
			} else if msg == "update" {
				getUpdate = true
			}
		}
		if getSnapshot && getUpdate {
			break
		}
	}
}

func TestBalance(t *testing.T) {
	accountsStr := os.Getenv("COINBASE_ACCOUNT_IDS")
	accounts := strings.Split(accountsStr, ",")

	c := make(chan struct{})

	t.Run("Run Balance", func(t *testing.T) {
		stream := getTestStreamOrSkip(t)
		for _, accountID := range accounts {
			stream.Subscribe("balance", accountID, types.SubscribeOptions{})
		}
		triggered := false
		stream.OnBalanceMessage(func(m *BalanceMessage) {
			if triggered {
				return
			}
			triggered = true
			assert.NotNil(t, m)
			// t.Logf("get balance message: %v", *m)
			c <- struct{}{}
		})
		err := stream.Connect(context.Background())
		assert.NoError(t, err)
	})
	<-c
}

func getTestStreamOrSkip(t *testing.T) *Stream {
	if isCI, _ := strconv.ParseBool(os.Getenv("CI")); isCI {
		t.Skip("skip test for CI")
	}

	key, secret, passphrase, ok := testutil.IntegrationTestWithPassphraseConfigured(t, "COINBASE")
	if !ok {
		t.Skip("COINBASE_* env vars not set")
	}
	exchange := New(key, secret, passphrase, 0)
	stream := NewStream(exchange, key, passphrase, secret)
	return stream
}
