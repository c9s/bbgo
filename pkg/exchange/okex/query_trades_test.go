package okex

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/c9s/bbgo/pkg/types"
	"github.com/stretchr/testify/assert"
)

func Test_QueryTrades(t *testing.T) {
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

	since := time.Now().AddDate(0, -3, 0)
	until := time.Now()

	queryOption := types.TradeQueryOptions{
		StartTime: &since,
		EndTime:   &until,
		Limit:     100,
	}
	transactionDetail, err := e.QueryTrades(context.Background(), queryOrder.Symbol, &queryOption)
	if assert.NoError(t, err) {
		assert.NotEmpty(t, transactionDetail)
	}
	t.Logf("transaction detail: %+v", transactionDetail)
}
