package binance

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAllSymbols(t *testing.T) {
	e := New("mock_key", "mock_secret")
	got, err := e.QueryTickers(context.Background())

	assert.NoError(t, err)

	if len(got) <= 1 {
		t.Errorf("Binance Exchange: Attempting to get all symbol tickers, but get 1 or less")
	}

}

func TestSomeSymbols(t *testing.T) {
	e := New("mock_key", "mock_secret")
	got, err := e.QueryTickers(context.Background(), "BTCUSDT", "ETHUSDT")

	assert.NoError(t, err)

	if len(got) != 2 {
		t.Errorf("Binance Exchange: Attempting to get two symbols, but number of tickers do not match")

	}
}

func TestSingleSymbol(t *testing.T) {
	e := New("mock_key", "mock_secret")
	got, err := e.QueryTickers(context.Background(), "BTCUSDT")

	assert.NoError(t, err)

	if len(got) != 1 {
		t.Errorf("Binance Exchange: Attempting to get one symbol, but number of tickers do not match")

	}
}
