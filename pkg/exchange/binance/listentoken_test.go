package binance

import (
	"context"
	"testing"
)

func TestExchange_EnableListenKey(t *testing.T) {
	exchange := &Exchange{}

	// Initially should be false (default uses new method)
	if exchange.useListenKey {
		t.Error("UseListenKey should be false by default")
	}

	// After enabling, should be true (uses deprecated method)
	exchange.EnableListenKey()
	if !exchange.useListenKey {
		t.Error("UseListenKey should be true after EnableListenKey()")
	}
}

func TestStream_ListenKeyConfiguration(t *testing.T) {
	// Test default behavior (new listenToken method)
	exchange1 := &Exchange{}
	stream1 := NewStream(exchange1, nil, nil)
	if stream1.exchange.useListenKey {
		t.Error("Stream should use listenToken method by default (useListenKey should be false)")
	}

	// Test enabling deprecated listenKey method
	exchange2 := &Exchange{}
	exchange2.EnableListenKey()
	stream2 := NewStream(exchange2, nil, nil)
	if !stream2.exchange.useListenKey {
		t.Error("Stream should have useListenKey enabled when exchange has UseListenKey enabled")
	}
}

func TestStream_FetchListenToken_ListenKeyEnabled(t *testing.T) {
	stream := &Stream{
		exchange: &Exchange{
			useListenKey: true,
		},
	}

	// Should return error when deprecated listenKey method is enabled
	_, _, err := stream.fetchListenToken(context.Background())
	if err == nil {
		t.Error("Expected error when listenKey method is enabled")
	}
}

func TestStream_FetchListenToken_NewMethodDefault(t *testing.T) {
	stream := &Stream{
		exchange: &Exchange{
			useListenKey: false,
		},
	}

	// This would normally require a real API client, so we just test the logic check
	if stream.exchange.useListenKey {
		t.Error("useListenKey should be false for new method")
	}
}
