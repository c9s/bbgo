package bbgo

import (
	"sync"
	"testing"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

func TestExchangeSession_LastPricesMutex_ConcurrentAccess(t *testing.T) {
	session := &ExchangeSession{
		lastPrices: make(map[string]fixedpoint.Value),
	}

	var wg sync.WaitGroup
	symbol := "BTCUSDT"
	writeCount := 50

	// Concurrently write to lastPrices
	for i := range writeCount {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			price := fixedpoint.NewFromInt(int64(i))
			session.setLastPrice(symbol, price)
		}(i)
	}

	// Concurrently read from lastPrices
	for range writeCount {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = session.LastPrice(symbol)
		}()
	}

	wg.Wait()

	price, ok := session.LastPrice(symbol)
	if !ok {
		t.Fatalf("expected price for symbol %s", symbol)
	}

	if price.Int64() < 0 || price.Int64() >= int64(writeCount) {
		t.Errorf("unexpected price %d, should be in [0, %d)", price.Int64(), writeCount)
	}
}
