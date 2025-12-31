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

// Integration test for the regular (non-algo) LIMIT order path in submitFuturesOrder.
// Mirrors the gating pattern used in Test_QueryPositionRisk.
func Test_SubmitFuturesOrder_Limit_Regular(t *testing.T) {
	key, secret, ok := testutil.IntegrationTestConfigured(t, "BINANCE")
	if !ok {
		t.SkipNow()
		return
	}

	ex := New(key, secret)
	ex.UseFutures()
	ctx := context.Background()

	// Load market metadata for BTCUSDT to format/truncate price correctly and avoid -1111
	markets, err := ex.QueryMarkets(ctx)
	require.NoError(t, err, "QueryMarkets should succeed for futures")
	mkt, ok := markets["BTCUSDT"]
	require.True(t, ok, "BTCUSDT market metadata should be available")

	// Get a current price and set a deep off-book LIMIT SELL to avoid fill
	tkr, err := ex.QueryTicker(ctx, "BTCUSDT")
	require.NoError(t, err, "QueryTicker should succeed for BTCUSDT")
	current := tkr.GetValidPrice()

	// 50% above current and truncate to tick size
	high := current.Mul(fixedpoint.NewFromInt(150)).Div(fixedpoint.NewFromInt(100))
	high = mkt.TruncatePrice(high)

	// Deep off-book LIMIT SELL to avoid fill; tiny qty to minimize reserved margin.
	submit := types.SubmitOrder{
		Symbol:   "BTCUSDT",
		Side:     types.SideTypeSell,
		Type:     types.OrderTypeLimit,
		Quantity: fixedpoint.MustNewFromString("0.001"),
		Price:    high,
		Market:   mkt, // pass market so the exchange uses correct precision formatting
	}

	created, err := ex.submitFuturesOrder(ctx, submit)
	require.NoError(t, err)
	require.NotNil(t, created)

	assert.Equal(t, types.OrderTypeLimit, created.Type)
	assert.Equal(t, types.SideTypeSell, created.Side)
	assert.Equal(t, "BTCUSDT", created.Symbol)
	assert.NotEmpty(t, created.ClientOrderID)
	assert.True(t, created.OrderID > 0, "expected valid OrderID")

	// Cleanup: cancel the created order by OrderID
	t.Cleanup(func() {
		_, _ = ex.futuresClient.NewCancelOrderService().
			Symbol(created.Symbol).
			OrderID(int64(created.OrderID)).
			Do(ctx)
	})
}
