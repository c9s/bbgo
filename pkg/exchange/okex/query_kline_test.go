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
	klineDetail, err := e.QueryKLines(context.Background(), queryOrder.Symbol, types.Interval1m, types.KLineQueryOptions{
		Limit:   50,
		EndTime: &now})
	if assert.NoError(t, err) {
		assert.NotEmpty(t, klineDetail)
	}
	// test supported interval - hour - 1 hour
	klineDetail, err = e.QueryKLines(context.Background(), queryOrder.Symbol, types.Interval1h, types.KLineQueryOptions{
		Limit:   50,
		EndTime: &now})
	if assert.NoError(t, err) {
		assert.NotEmpty(t, klineDetail)
	}
	// test supported interval - hour - 6 hour to test UTC time
	klineDetail, err = e.QueryKLines(context.Background(), queryOrder.Symbol, types.Interval6h, types.KLineQueryOptions{
		Limit:   50,
		EndTime: &now})
	if assert.NoError(t, err) {
		assert.NotEmpty(t, klineDetail)
	}
	// test supported interval - day
	klineDetail, err = e.QueryKLines(context.Background(), queryOrder.Symbol, types.Interval1d, types.KLineQueryOptions{
		Limit:   50,
		EndTime: &now})
	if assert.NoError(t, err) {
		assert.NotEmpty(t, klineDetail)
		assert.NotEmpty(t, klineDetail[0].Exchange)
		assert.NotEmpty(t, klineDetail[0].Symbol)
		assert.NotEmpty(t, klineDetail[0].StartTime)
		assert.NotEmpty(t, klineDetail[0].EndTime)
		assert.NotEmpty(t, klineDetail[0].Interval)
		assert.NotEmpty(t, klineDetail[0].Open)
		assert.NotEmpty(t, klineDetail[0].Close)
		assert.NotEmpty(t, klineDetail[0].High)
		assert.NotEmpty(t, klineDetail[0].Low)
		assert.NotEmpty(t, klineDetail[0].Volume)
	}
	// test supported interval - week
	klineDetail, err = e.QueryKLines(context.Background(), queryOrder.Symbol, types.Interval1w, types.KLineQueryOptions{
		Limit:   50,
		EndTime: &now})
	if assert.NoError(t, err) {
		assert.NotEmpty(t, klineDetail)
	}
	// test supported interval - month
	klineDetail, err = e.QueryKLines(context.Background(), queryOrder.Symbol, types.Interval1mo, types.KLineQueryOptions{
		Limit:   50,
		EndTime: &now})
	if assert.NoError(t, err) {
		assert.NotEmpty(t, klineDetail)
	}
	// test not supported interval
	klineDetail, err = e.QueryKLines(context.Background(), queryOrder.Symbol, types.Interval("2m"), types.KLineQueryOptions{
		Limit:   50,
		EndTime: &now})
	if assert.Error(t, err) {
		assert.Empty(t, klineDetail)
	}
}
