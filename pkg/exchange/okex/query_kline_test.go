package okex

import (
	"context"
	"testing"
	"time"

	"github.com/c9s/bbgo/pkg/testutil"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/stretchr/testify/assert"
)

func Test_QueryKlines(t *testing.T) {
	key, secret, passphrase, ok := testutil.IntegrationTestWithPassphraseConfigured(t, "OKEX")
	if !ok {
		t.Skip("Please configure all credentials about OKEX")
	}

	e := New(key, secret, passphrase)

	queryOrder := types.OrderQuery{
		Symbol: "BTC-USDT",
	}

	now := time.Now()
	// test supported interval - minute
	klineDetail, err := e.QueryKLines(context.Background(), queryOrder.Symbol, types.Interval("1m"), types.KLineQueryOptions{
		Limit:   50,
		EndTime: &now})
	if assert.NoError(t, err) {
		assert.NotEmpty(t, klineDetail)
	}
	t.Logf("kline detail: %+v", klineDetail)
	// test supported interval - hour
	klineDetail, err = e.QueryKLines(context.Background(), queryOrder.Symbol, types.Interval("1h"), types.KLineQueryOptions{
		Limit:   50,
		EndTime: &now})
	if assert.NoError(t, err) {
		assert.NotEmpty(t, klineDetail)
	}
	t.Logf("kline detail: %+v", klineDetail)
	// test supported interval - day
	klineDetail, err = e.QueryKLines(context.Background(), queryOrder.Symbol, types.Interval("1d"), types.KLineQueryOptions{
		Limit:   50,
		EndTime: &now})
	if assert.NoError(t, err) {
		assert.NotEmpty(t, klineDetail)
	}
	t.Logf("kline detail: %+v", klineDetail)
	// test supported interval - week
	klineDetail, err = e.QueryKLines(context.Background(), queryOrder.Symbol, types.Interval("1w"), types.KLineQueryOptions{
		Limit:   50,
		EndTime: &now})
	if assert.NoError(t, err) {
		assert.NotEmpty(t, klineDetail)
	}
	t.Logf("kline detail: %+v", klineDetail)
	// test supported interval - month
	klineDetail, err = e.QueryKLines(context.Background(), queryOrder.Symbol, types.Interval("1mo"), types.KLineQueryOptions{
		Limit:   50,
		EndTime: &now})
	if assert.NoError(t, err) {
		assert.NotEmpty(t, klineDetail)
	}
	t.Logf("kline detail: %+v", klineDetail)
	// test not supported interval
	klineDetail, err = e.QueryKLines(context.Background(), queryOrder.Symbol, types.Interval("2m"), types.KLineQueryOptions{
		Limit:   50,
		EndTime: &now})
	if assert.Error(t, err) {
		assert.Empty(t, klineDetail)
	}
	t.Logf("kline error log: %+v", err)
}
