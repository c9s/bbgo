package ftxapi

import (
	"context"
	"os"
	"regexp"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

func maskSecret(s string) string {
	re := regexp.MustCompile(`\b(\w{4})\w+\b`)
	s = re.ReplaceAllString(s, "$1******")
	return s
}

func integrationTestConfigured(t *testing.T) (key, secret string, ok bool) {
	var hasKey, hasSecret bool
	key, hasKey = os.LookupEnv("FTX_API_KEY")
	secret, hasSecret = os.LookupEnv("FTX_API_SECRET")
	ok = hasKey && hasSecret && os.Getenv("TEST_FTX") == "1"
	if ok {
		t.Logf("ftx api integration test enabled, key = %s, secret = %s", maskSecret(key), maskSecret(secret))
	}
	return key, secret, ok
}

func TestClient_Requests(t *testing.T) {
	key, secret, ok := integrationTestConfigured(t)
	if !ok {
		t.SkipNow()
		return
	}

	ctx, cancel := context.WithTimeout(context.TODO(), 15*time.Second)
	defer cancel()

	client := NewClient()
	client.Auth(key, secret, "")

	testCases := []struct {
		name string
		tt   func(t *testing.T)
	}{
		{
			name: "GetMarketsRequest",
			tt: func(t *testing.T) {
				req := client.NewGetMarketsRequest()
				markets, err := req.Do(ctx)
				assert.NoError(t, err)
				assert.NotNil(t, markets)
				t.Logf("markets: %+v", markets)
			},
		},
		{
			name: "GetAccountRequest",
			tt: func(t *testing.T) {
				req := client.NewGetAccountRequest()
				account, err := req.Do(ctx)
				assert.NoError(t, err)
				assert.NotNil(t, account)
				t.Logf("account: %+v", account)
			},
		},
		{
			name: "PlaceOrderRequest",
			tt: func(t *testing.T) {
				req := client.NewPlaceOrderRequest()
				req.PostOnly(true).
					Size(fixedpoint.MustNewFromString("1.0")).
					Price(fixedpoint.MustNewFromString("10.0")).
					OrderType(OrderTypeLimit).
					Side(SideBuy).
					Market("LTC/USDT")

				createdOrder, err := req.Do(ctx)
				if assert.NoError(t, err) {
					assert.NotNil(t, createdOrder)
					t.Logf("createdOrder: %+v", createdOrder)

					req2 := client.NewCancelOrderRequest(strconv.FormatInt(createdOrder.Id, 10))
					ret, err := req2.Do(ctx)
					assert.NoError(t, err)
					t.Logf("cancelOrder: %+v", ret)
					assert.True(t, ret.Success)
				}
			},
		},
		{
			name: "GetFillsRequest",
			tt: func(t *testing.T) {
				req := client.NewGetFillsRequest()
				req.Market("CRO/USD")
				fills, err := req.Do(ctx)
				assert.NoError(t, err)
				assert.NotNil(t, fills)
				t.Logf("fills: %+v", fills)
			},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, testCase.tt)
	}
}
