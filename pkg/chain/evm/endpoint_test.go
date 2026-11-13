package evm

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewEndpointPool(t *testing.T) {
	_, err := NewEndpointPool(nil)
	assert.Error(t, err)

	_, err = NewEndpointPool([]string{"  ", ""})
	assert.Error(t, err)

	pool, err := NewEndpointPool([]string{" https://rpc.example/key123 ", "https://backup.example"})
	assert.NoError(t, err)
	assert.Equal(t, 2, pool.Len())
	assert.Equal(t, "https://rpc.example/***", pool.Current())
	assert.Equal(t, []string{"https://rpc.example/***", "https://backup.example"}, pool.Endpoints())
}

func TestRedactEndpoint(t *testing.T) {
	cases := map[string]string{
		"https://mainnet.infura.io/v3/abcdef":    "https://mainnet.infura.io/v3/***",
		"https://bsc-dataseed.binance.org":       "https://bsc-dataseed.binance.org",
		"https://rpc.example/path?apikey=secret": "https://rpc.example/***?***",
		"wss://user:pass@rpc.example/ws":         "wss://***@rpc.example/***",
	}

	for input, expected := range cases {
		assert.Equal(t, expected, RedactEndpoint(input), "input: %s", input)
	}
}

func TestIsRetryable(t *testing.T) {
	assert.False(t, IsRetryable(nil))
	assert.False(t, IsRetryable(context.Canceled))
	assert.False(t, IsRetryable(context.DeadlineExceeded))
	assert.False(t, IsRetryable(errors.New("execution reverted: STF")))
	assert.False(t, IsRetryable(errors.New("nonce too low")))
	assert.True(t, IsRetryable(errors.New("429 Too Many Requests")))
	assert.True(t, IsRetryable(errors.New("connection reset by peer")))
}
