package binance

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/testutil"
	"github.com/c9s/bbgo/pkg/types"
)

// Test_SubmitFuturesOrder_StopMarket_Algo
//
// This integration test verifies that submitFuturesOrder correctly routes
// STOP_MARKET orders through Binance Futures Algo endpoint and returns a
// consistent Order object. It uses production API keys if configured
// via testutil.IntegrationTestConfigured and skips otherwise.
func Test_SubmitFuturesOrder_StopMarket_Algo(t *testing.T) {
	key, secret, ok := testutil.IntegrationTestConfigured(t, "BINANCE")
	if !ok {
		t.Skip("BINANCE integration credentials not configured")
		return
	}

	ctx := context.Background()

	ex := New(key, secret)
	ex.UseFutures()

	// Query latest ticker to determine a safe stop price around -2% from current
	tkr, err := ex.QueryTicker(ctx, "BTCUSDT")
	require.NoError(t, err, "QueryTicker should succeed for BTCUSDT")
	current := tkr.GetValidPrice()

	// Load markets and fetch the BTCUSDT market for proper tick/step info
	markets, err := ex.QueryMarkets(ctx)
	require.NoError(t, err, "QueryMarkets should succeed for futures")
	mkt, ok := markets["BTCUSDT"]
	require.True(t, ok, "BTCUSDT market metadata should be available")

	// For a SELL stop-market, place trigger ~2% below current to avoid immediate trigger.
	// Truncate to tick size using market info to avoid Binance -1111 precision error.
	stop := current.Mul(fixedpoint.NewFromInt(98)).Div(fixedpoint.NewFromInt(100))
	stop = mkt.TruncatePrice(stop)

	// Use BTCUSDT with a tiny quantity and a trigger price derived from the
	// latest ticker (~ -2%) to satisfy Binance's requirement that the stop
	// price is not too far from the current market price. We intentionally
	// leave Market empty so the builder falls back to FormatString(8), which
	// is acceptable for the common BTCUSDT step settings.
	submit := types.SubmitOrder{
		Symbol:    "BTCUSDT",
		Side:      types.SideTypeSell,
		Type:      types.OrderTypeStopMarket,
		Quantity:  mkt.MinQuantity,
		StopPrice: stop,
		Market:    mkt, // pass market so the exchange uses correct precision formatting
	}

	// Place the order via the unified submit method (algo path expected)
	created, err := ex.submitFuturesOrder(ctx, submit)
	if !assert.NoError(t, err, "submitFuturesOrder should succeed for STOP_MARKET via algo endpoint") {
		return
	}
	if !assert.NotNil(t, created, "created order should not be nil") {
		return
	}

	// Basic field validations
	assert.Equal(t, types.OrderTypeStopMarket, created.Type, "order type should be STOP_MARKET")
	assert.Equal(t, submit.Symbol, created.Symbol)
	assert.Equal(t, submit.Side, created.Side)
	assert.True(t, created.IsFutures)
	assert.Equal(t, types.OrderStatusNew, created.Status)
	assert.NotEmpty(t, created.ClientOrderID, "client algo id (client order id) should be set")

	// Cleanup: cancel the algo order by client algo id to avoid leaving state
	// We are in the same package, so accessing ex.futuresClient is allowed.
	t.Cleanup(func() {
		if created != nil && created.ClientOrderID != "" {
			// Best-effort cleanup; do not fail the test on cleanup error
			_, _ = ex.futuresClient.NewCancelAlgoOrderService().ClientAlgoID(created.ClientOrderID).Do(ctx)
		}
	})
}
