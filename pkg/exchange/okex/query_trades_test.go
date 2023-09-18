package okex

import (
	"context"
	"testing"
	"time"

	"github.com/c9s/bbgo/pkg/testutil"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/stretchr/testify/assert"
)

func Test_QueryTrades(t *testing.T) {
	key, secret, passphrase, ok := testutil.IntegrationTestWithPassphraseConfigured(t, "OKEX")
	if !ok {
		t.Skip("Please configure all credentials about OKEX")
	}

	e := New(key, secret, passphrase)

	queryOrder := types.OrderQuery{
		Symbol: "BTCUSDT",
	}

	since := time.Now().AddDate(0, -3, 0)
	until := time.Now()

	queryOption := types.TradeQueryOptions{
		StartTime: &since,
		EndTime:   &until,
		Limit:     100,
	}
	// query by time interval
	transactionDetail, err := e.QueryTrades(context.Background(), queryOrder.Symbol, &queryOption)
	if assert.NoError(t, err) {
		assert.NotEmpty(t, transactionDetail)
	}
	t.Logf("transaction detail: %+v", transactionDetail)
	// query by trade id
	transactionDetail, err = e.QueryTrades(context.Background(), queryOrder.Symbol, &types.TradeQueryOptions{LastTradeID: 432044402})
	if assert.NoError(t, err) {
		assert.NotEmpty(t, transactionDetail)
	}
	t.Logf("transaction detail: %+v", transactionDetail)
	// query by no time interval and no trade id
	transactionDetail, err = e.QueryTrades(context.Background(), queryOrder.Symbol, &types.TradeQueryOptions{})
	if assert.NoError(t, err) {
		assert.NotEmpty(t, transactionDetail)
	}
	t.Logf("transaction detail: %+v", transactionDetail)
	// query by limit exceed default value
	transactionDetail, err = e.QueryTrades(context.Background(), queryOrder.Symbol, &types.TradeQueryOptions{Limit: 150})
	if assert.NoError(t, err) {
		assert.NotEmpty(t, transactionDetail)
	}
	t.Logf("transaction detail: %+v", transactionDetail)
}
