package max

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	. "github.com/c9s/bbgo/pkg/testing/testhelper"
	"github.com/c9s/bbgo/pkg/testutil"
	"github.com/c9s/bbgo/pkg/types"
)

func TestExchange_recoverOrder(t *testing.T) {
	key, secret, ok := testutil.IntegrationTestConfigured(t, "MAX")
	if !ok {
		t.SkipNow()
	}

	ctx := context.Background()
	ex := New(key, secret)

	_, err := ex.recoverOrder(ctx, types.SubmitOrder{
		Symbol:        "BTCUSDT",
		ClientOrderID: "test" + strconv.FormatInt(time.Now().UnixMilli(), 10),
	})
	t.Logf("recover order error: %v", err)
	assert.Nil(t, err, "order should be nil if not found")

	orderForm := types.SubmitOrder{
		Symbol:        "BTCUSDT",
		ClientOrderID: "test" + strconv.FormatInt(time.Now().UnixMilli(), 10),
		Type:          types.OrderTypeLimit,
		Side:          types.SideTypeBuy,
		Price:         Number(90_000),
		Quantity:      Number(0.001),
	}

	order, err := ex.recoverOrder(ctx, orderForm)

	if assert.NoError(t, err) {
		t.Logf("order: %+v", order)
		assert.Nil(t, order, "order should be nil if not found")

		order, err = ex.SubmitOrder(ctx, orderForm)
		if assert.NoError(t, err) {
			t.Logf("submitted order: %+v", order)
			if assert.NotNil(t, order) {
				err = ex.CancelOrders(ctx, *order)
				assert.NoError(t, err)

				order2, err2 := ex.recoverOrder(ctx, orderForm)

				t.Logf("recovered order: %+v", order2)
				assert.NoError(t, err2)
				assert.NotNil(t, order2)
			}
		}
	}
}
