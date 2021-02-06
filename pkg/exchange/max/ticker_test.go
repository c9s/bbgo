package max

import (
	"context"
	"fmt"
	"testing"
)

func TestAllSymbols(t *testing.T) {
	e := New("mock_key", "mock_secret")
	got, err := e.QueryTickers(context.Background())
	if err != nil {
		t.Errorf("Max Exchange: Fail to get ticker for all symbols: %s", err)
	}
	if len(got) <= 1 {
		t.Errorf("Max Exchange: Attempting to get all symbol tickers, but get 1 or less")
	}

}

func TestSomeSymbols(t *testing.T) {
	e := New("mock_key", "mock_secret")
	got, err := e.QueryTickers(context.Background(), "BTCUSDT", "ETHUSDT")

	if err != nil {
		t.Errorf("Max Exchange: Fail to get ticker for some symbols: %s", err)
	}

	if len(got) != 2 {
		fmt.Println(got)
		t.Errorf("Max Exchange: Attempting to get two symbols, but number of tickers do not match")

	}
}

func TestSingleSymbol(t *testing.T) {
	e := New("mock_key", "mock_secret")
	got, err := e.QueryTickers(context.Background(), "BTCUSDT")
	if err != nil {
		t.Errorf("Max Exchange: Fail to get ticker for single symbol: %s", err)
	}

	if len(got) != 1 {
		fmt.Println(got)
		t.Errorf("Max Exchange: Attempting to get one symbol, but number of tickers do not match")

	}
}
