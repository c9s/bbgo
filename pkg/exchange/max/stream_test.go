package max

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/testutil"
	"github.com/c9s/bbgo/pkg/types"
)

func TestStream_Public(t *testing.T) {
	key, secret, ok := testutil.IntegrationTestConfigured(t, "MAX")
	if !ok {
		t.SkipNow()
	}

	// Create a MAX exchange instance (can use empty credentials for public stream)
	ex := New(key, secret, "")

	// Create a public data stream
	stream := NewStream(ex)
	stream.SetPublicOnly()

	// Subscribe to kline channel
	stream.Subscribe(types.KLineChannel, "btcusdt", types.SubscribeOptions{
		Interval: types.Interval1m,
	})

	// Set up a channel to signal when we receive kline data
	klineReceived := make(chan types.KLine, 1)

	// Set up kline callback
	stream.OnKLine(func(kline types.KLine) {
		t.Logf("received kline: %+v", kline)
		klineReceived <- kline
	})

	// Create a context
	ctx := context.Background()

	// Connect and check there is no error
	err := stream.Connect(ctx)
	assert.NoError(t, err, "stream connection should not error")

	// Wait at most 15 seconds until getting the first kline feed or test fail
	select {
	case kline := <-klineReceived:
		t.Logf("successfully received kline: symbol=%s, interval=%s, open=%s, close=%s",
			kline.Symbol, kline.Interval, kline.Open.String(), kline.Close.String())
		assert.NotEmpty(t, kline.Symbol, "kline symbol should not be empty")
		assert.NotZero(t, kline.StartTime, "kline start time should not be zero")
	case <-time.After(15 * time.Second):
		t.Fatal("timeout waiting for kline data")
	}
}

func TestStream_Private(t *testing.T) {
	key, secret, ok := testutil.IntegrationTestConfigured(t, "MAX")
	if !ok {
		t.SkipNow()
	}

	// Create a MAX exchange instance with credentials
	ex := New(key, secret, "")

	// Create a private data stream
	stream := NewStream(ex)

	connectReceived := make(chan struct{}, 1)

	// Set up connect callback
	stream.OnConnect(func() {
		t.Log("stream connected")
		connectReceived <- struct{}{}
	})

	// Create a context
	ctx := context.Background()

	// Connect and check there is no error
	err := stream.Connect(ctx)
	assert.NoError(t, err, "stream connection should not error")

	// Wait at most 30 seconds until getting the private stream connection or test fail
	select {
	case <-connectReceived:
		t.Log("successfully connected to private stream")
	case <-time.After(30 * time.Second):
		t.Fatal("timeout waiting for private stream connection")
	}
}
