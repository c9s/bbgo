package ftx

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExchange_QueryTickers_AllSymbols(t *testing.T) {
	key := os.Getenv("FTX_API_KEY")
	secret := os.Getenv("FTX_API_SECRET")
	subAccount := os.Getenv("FTX_SUBACCOUNT")
	if len(key) == 0 && len(secret) == 0 {
		t.Skip("api key/secret are not configured")
	}

	e := NewExchange(key, secret, subAccount)
	got, err := e.QueryTickers(context.Background())
	if assert.NoError(t, err) {
		assert.True(t, len(got) > 1, "binance: attempting to get all symbol tickers, but get 1 or less")
	}
}

func TestExchange_QueryTickers_SomeSymbols(t *testing.T) {
	key := os.Getenv("FTX_API_KEY")
	secret := os.Getenv("FTX_API_SECRET")
	subAccount := os.Getenv("FTX_SUBACCOUNT")
	if len(key) == 0 && len(secret) == 0 {
		t.Skip("api key/secret are not configured")
	}

	e := NewExchange(key, secret, subAccount)
	got, err := e.QueryTickers(context.Background(), "BTCUSDT", "ETHUSDT")
	if assert.NoError(t, err) {
		assert.Len(t, got, 2, "binance: attempting to get two symbols, but number of tickers do not match")
	}
}

func TestExchange_QueryTickers_SingleSymbol(t *testing.T) {
	key := os.Getenv("FTX_API_KEY")
	secret := os.Getenv("FTX_API_SECRET")
	subAccount := os.Getenv("FTX_SUBACCOUNT")
	if len(key) == 0 && len(secret) == 0 {
		t.Skip("api key/secret are not configured")
	}

	e := NewExchange(key, secret, subAccount)
	got, err := e.QueryTickers(context.Background(), "BTCUSDT")
	if assert.NoError(t, err) {
		assert.Len(t, got, 1, "binance: attempting to get one symbol, but number of tickers do not match")
	}
}
