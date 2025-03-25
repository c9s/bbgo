package coinbase

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/testutil"
	"github.com/c9s/bbgo/pkg/types"
)

func Test_new(t *testing.T) {
	ex := getExchangeOrSkip(t)
	assert.Equal(t, ex.Name(), types.ExchangeCoinBase)
	t.Log("successfully created coinbase exchange client")
	_ = ex.SupportedInterval()
	_ = ex.PlatformFeeCurrency()
}

func Test_Symbols(t *testing.T) {
	globalSymbol := "NOTEXIST"
	localSymbol := toLocalSymbol(globalSymbol)
	assert.Equal(t, globalSymbol, toGlobalSymbol(localSymbol))
	assert.Equal(t, localSymbol, toLocalSymbol(globalSymbol))

	globalSymbol = "ETHUSD"
	localSymbol = toLocalSymbol(globalSymbol)
	assert.Equal(t, globalSymbol, toGlobalSymbol(localSymbol))
	assert.Equal(t, localSymbol, toLocalSymbol(globalSymbol))
}

func Test_OrdersAPI(t *testing.T) {
	ex := getExchangeOrSkip(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	// should fail on unsupported symbol
	order, err := ex.SubmitOrder(
		ctx,
		types.SubmitOrder{
			Market: types.Market{
				Symbol: "NOTEXIST",
			},
			Side:     types.SideTypeBuy,
			Type:     types.OrderTypeLimit,
			Price:    fixedpoint.MustNewFromString("0.001"),
			Quantity: fixedpoint.MustNewFromString("0.001"),
		})
	assert.Error(t, err)
	assert.Empty(t, order)
	// should succeed
	symbol := "ETHUSD"
	order, err = ex.SubmitOrder(
		ctx,
		types.SubmitOrder{
			Market: types.Market{
				Symbol: symbol,
			},
			Side:     types.SideTypeBuy,
			Type:     types.OrderTypeLimit,
			Price:    fixedpoint.MustNewFromString("0.01"),
			Quantity: fixedpoint.MustNewFromString("100"), // min funds is $1
		})
	assert.NoError(t, err)
	assert.NotEmpty(t, order)

	// test query open orders
	order, err = ex.QueryOrder(ctx, types.OrderQuery{Symbol: symbol, OrderID: order.UUID, ClientOrderID: order.UUID})
	assert.NoError(t, err)

	// the status might be pending at the beginning. Wait until it is open
	// only retry 5 times
	for i := 0; i < 5; i++ {
		if order.OriginalStatus == "open" {
			break
		}
		time.Sleep(time.Millisecond * 500)
		order, err = ex.QueryOrder(ctx, types.OrderQuery{Symbol: symbol, OrderID: order.UUID, ClientOrderID: order.UUID})
		assert.NoError(t, err)
	}

	orders, err := ex.QueryOpenOrders(ctx, symbol)
	assert.NoError(t, err)
	found := false
	for _, o := range orders {
		if o.UUID == order.UUID {
			found = true
			break
		}
	}
	assert.True(t, found)

	// test cancel order
	err = ex.CancelOrders(ctx, types.Order{
		Exchange: types.ExchangeCoinBase,
		UUID:     order.UUID,
	})
	assert.NoError(t, err)
}

func Test_QueryAccount(t *testing.T) {
	ex := getExchangeOrSkip(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	_, err := ex.QueryAccount(ctx)
	assert.NoError(t, err)
}

func Test_QueryAccountBalances(t *testing.T) {
	ex := getExchangeOrSkip(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	_, err := ex.QueryAccountBalances(ctx)
	assert.NoError(t, err)
}

func Test_QueryOpenOrders(t *testing.T) {
	ex := getExchangeOrSkip(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	symbols := []string{"BTCUSD", "ETHUSD", "ETHBTC"}
	for _, k := range symbols {
		_, err := ex.QueryOpenOrders(ctx, k)
		assert.NoError(t, err)
	}
}

func Test_QueryMarkets(t *testing.T) {
	ex := getExchangeOrSkip(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	_, err := ex.QueryMarkets(ctx)
	assert.NoError(t, err)
}

func Test_QueryTicker(t *testing.T) {
	ex := getExchangeOrSkip(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	ticker, err := ex.QueryTicker(ctx, "BTCUSD")
	assert.NoError(t, err)
	assert.NotNil(t, ticker)
}

func Test_QueryTickers(t *testing.T) {
	ex := getExchangeOrSkip(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	symbols := []string{"BTCUSD", "ETHUSD", "ETHBTC"}
	tickers, err := ex.QueryTickers(ctx, symbols...)
	assert.NoError(t, err)
	assert.NotNil(t, tickers)
}

func Test_QueryKLines(t *testing.T) {
	ex := getExchangeOrSkip(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	// should fail on unsupported interval
	_, err := ex.QueryKLines(ctx, "BTCUSD", types.Interval12h, types.KLineQueryOptions{})
	assert.Error(t, err)

	klines, err := ex.QueryKLines(ctx, "BTCUSD", types.Interval1m, types.KLineQueryOptions{})
	assert.NoError(t, err)
	assert.NotNil(t, klines)

	endTime := time.Now()
	startTime := endTime.Add(-time.Hour * 5)
	klines, err = ex.QueryKLines(
		ctx,
		"BTCUSD",
		types.Interval1m,
		types.KLineQueryOptions{
			StartTime: &startTime,
			EndTime:   &endTime,
		},
	)
	assert.NoError(t, err)
	assert.NotNil(t, klines)
}

func Test_QueryOrderTrades(t *testing.T) {
	ex := getExchangeOrSkip(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	trades, err := ex.QueryOrderTrades(ctx, types.OrderQuery{Symbol: "ETHUSD"})
	assert.NoError(t, err)
	assert.NotNil(t, trades)
}

func getExchangeOrSkip(t *testing.T) *Exchange {
	if b, _ := strconv.ParseBool(os.Getenv("CI")); b {
		t.Skip("skip test for CI")
	}
	key, secret, passphrase, ok := testutil.IntegrationTestWithPassphraseConfigured(t, "COINBASE")
	if !ok {
		t.SkipNow()
		return nil
	}

	return New(key, secret, passphrase, 0)
}
