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
	// query by trade id
	transactionDetail, err = e.QueryTrades(context.Background(), queryOrder.Symbol, &types.TradeQueryOptions{LastTradeID: 432044402})
	if assert.Error(t, err) {
		assert.Empty(t, transactionDetail)
	}
	// query by no time interval and no trade id
	transactionDetail, err = e.QueryTrades(context.Background(), queryOrder.Symbol, &types.TradeQueryOptions{})
	if assert.Error(t, err) {
		assert.Empty(t, transactionDetail)
	}
	// query by limit exceed default value
	transactionDetail, err = e.QueryTrades(context.Background(), queryOrder.Symbol, &types.TradeQueryOptions{Limit: 150})
	if assert.Error(t, err) {
		assert.Empty(t, transactionDetail)
	}
	// pagenation test and test time interval : only end time
	transactionDetail, err = e.QueryTrades(context.Background(), queryOrder.Symbol, &types.TradeQueryOptions{EndTime: &until, Limit: 1})
	if assert.NoError(t, err) {
		assert.NotEmpty(t, transactionDetail)
		assert.Less(t, 1, len(transactionDetail))
	}
	// query by time interval: only start time
	transactionDetail, err = e.QueryTrades(context.Background(), queryOrder.Symbol, &types.TradeQueryOptions{StartTime: &since, Limit: 100})
	if assert.NoError(t, err) {
		assert.NotEmpty(t, transactionDetail)
	}
	// query by combination: start time, end time and after
	transactionDetail, err = e.QueryTrades(context.Background(), queryOrder.Symbol, &types.TradeQueryOptions{StartTime: &since, EndTime: &until, Limit: 1})
	if assert.NoError(t, err) {
		assert.NotEmpty(t, transactionDetail)
	}
	// query by time interval: 3 months earlier with start time and end time
	since = time.Now().AddDate(0, -6, 0)
	until = time.Now().AddDate(0, -3, 0)
	transactionDetail, err = e.QueryTrades(context.Background(), queryOrder.Symbol, &types.TradeQueryOptions{StartTime: &since, EndTime: &until, Limit: 100})
	if assert.NoError(t, err) {
		assert.Empty(t, transactionDetail)
	}
	// query by time interval: 3 months earlier with start time
	since = time.Now().AddDate(0, -6, 0)
	transactionDetail, err = e.QueryTrades(context.Background(), queryOrder.Symbol, &types.TradeQueryOptions{StartTime: &since, Limit: 100})
	if assert.NoError(t, err) {
		assert.NotEmpty(t, transactionDetail)
	}
}
