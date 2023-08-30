package okex

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/c9s/bbgo/pkg/types"
	"github.com/stretchr/testify/assert"
)

func Test_QueryKlines(t *testing.T) {
	key := os.Getenv("OKEX_API_KEY")
	secret := os.Getenv("OKEX_API_SECRET")
	passphrase := os.Getenv("OKEX_API_PASSPHRASE")
	if len(key) == 0 && len(secret) == 0 {
		t.Skip("api key/secret are not configured")
		return
	}
	if len(passphrase) == 0 {
		t.Skip("passphrase are not configured")
		return
	}

	e, err := New(key, secret, passphrase)
	assert.NoError(t, err)

	queryOrder := types.OrderQuery{
		Symbol: "BTC-USDT",
	}

	now := time.Now()
	klineDetail, err := e.QueryKLines(context.Background(), queryOrder.Symbol, types.Interval("1m"), types.KLineQueryOptions{
		Limit:   50,
		EndTime: &now})
	if assert.NoError(t, err) {
		assert.NotEmpty(t, klineDetail)
	}
	t.Logf("kline detail: %+v", klineDetail)
}
