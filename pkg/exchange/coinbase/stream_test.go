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

func TestStream(t *testing.T) {
	c := make(chan any)
	defer close(c)

	t.Run("Test Status", func(t *testing.T) {
		stream := getTestStreamOrSkip(t)
		stream.Subscribe("status", "", types.SubscribeOptions{})
		stream.OnStatusMessage(func(m *StatusMessage) {
			assert.NotNil(t, m)
			t.Logf("get status message: %v", *m)
			c <- m
		})
		err := stream.Connect(context.Background())
		assert.NoError(t, err)
	})
	<-c

	t.Run("Test Ticker", func(t *testing.T) {
		stream := getTestStreamOrSkip(t)
		stream.Subscribe("ticker", "BTC-USD", types.SubscribeOptions{})
		stream.OnTickerMessage(func(m *TickerMessage) {
			assert.NotNil(t, m)
			t.Logf("get ticker message: %v", *m)
			c <- m
		})
		err := stream.Connect(context.Background())
		assert.NoError(t, err)
	})
	<-c

	t.Run("Test Match", func(t *testing.T) {
		stream := getTestStreamOrSkip(t)
		stream.Subscribe("matches", "BTC-USD", types.SubscribeOptions{})
		stream.OnMatchMessage(func(m *MatchMessage) {
			assert.NotNil(t, m)
			t.Logf("get match message: %v", *m)
			c <- m
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
