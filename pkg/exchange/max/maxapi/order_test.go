package max

import (
	"context"
	"os"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

func maskSecret(s string) string {
	re := regexp.MustCompile(`\b(\w{4})\w+\b`)
	s = re.ReplaceAllString(s, "$1******")
	return s
}

func integrationTestConfigured(t *testing.T, prefix string) (key, secret string, ok bool) {
	var hasKey, hasSecret bool
	key, hasKey = os.LookupEnv(prefix + "_API_KEY")
	secret, hasSecret = os.LookupEnv(prefix + "_API_SECRET")
	ok = hasKey && hasSecret && os.Getenv("TEST_"+prefix) == "1"
	if ok {
		t.Logf(prefix+" api integration test enabled, key = %s, secret = %s", maskSecret(key), maskSecret(secret))
	}
	return key, secret, ok
}

func TestOrderService_GetOrdersRequest(t *testing.T) {
	key, secret, ok := integrationTestConfigured(t, "MAX")
	if !ok {
		t.SkipNow()
	}

	ctx := context.Background()

	client := NewRestClient(ProductionAPIURL)
	client.Auth(key, secret)

	req3 := client.OrderService.NewGetOrdersRequest()
	req3.State([]OrderState{OrderStateDone, OrderStateFinalizing})
	// req3.State([]OrderState{OrderStateDone})
	req3.Market("btcusdt")
	orders, err := req3.Do(ctx)
	if assert.NoError(t, err) {
		t.Logf("orders: %+v", orders)

		assert.NotNil(t, orders)
		if assert.NotEmptyf(t, orders, "got %d orders", len(orders)) {
			for _, order := range orders {
				assert.Contains(t, []OrderState{OrderStateDone, OrderStateFinalizing}, order.State)
			}
		}
	}
}

func TestOrderService_GetOrdersRequest_SingleState(t *testing.T) {
	key, secret, ok := integrationTestConfigured(t, "MAX")
	if !ok {
		t.SkipNow()
	}

	ctx := context.Background()

	client := NewRestClient(ProductionAPIURL)
	client.Auth(key, secret)

	req3 := client.OrderService.NewGetOrdersRequest()
	req3.State([]OrderState{OrderStateDone})
	req3.Market("btcusdt")
	orders, err := req3.Do(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, orders)
}

func TestOrderService_GetOrderHistoryRequest(t *testing.T) {
	key, secret, ok := integrationTestConfigured(t, "MAX")
	if !ok {
		t.SkipNow()
	}

	ctx := context.Background()

	client := NewRestClient(ProductionAPIURL)
	client.Auth(key, secret)

	req := client.OrderService.NewGetOrderHistoryRequest()
	req.Market("btcusdt")
	req.FromID(1)
	orders, err := req.Do(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, orders)
}

func TestOrderService(t *testing.T) {
	key, secret, ok := integrationTestConfigured(t, "MAX")
	if !ok {
		t.SkipNow()
	}

	ctx := context.Background()

	client := NewRestClient(ProductionAPIURL)
	client.Auth(key, secret)
	req := client.OrderService.NewCreateOrderRequest()
	order, err := req.Market("btcusdt").
		Price("10000").
		Volume("0.001").
		OrderType("limit").
		Side("buy").Do(ctx)

	if assert.NoError(t, err) {
		assert.NotNil(t, order)
		req2 := client.OrderService.NewOrderCancelRequest()
		req2.Id(order.ID)
		cancelResp, err := req2.Do(ctx)
		assert.NoError(t, err)
		t.Logf("cancelResponse: %+v", cancelResp)
	}

}
