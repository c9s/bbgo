package coinbase

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/testing/httptesting"
	"github.com/c9s/bbgo/pkg/testutil"
	"github.com/c9s/bbgo/pkg/types"
)

func TestExchange_new(t *testing.T) {
	ex, saveRecord := getExchangeOrSkip(t)
	defer saveRecord()

	assert.Equal(t, ex.Name(), types.ExchangeCoinBase)
	t.Log("successfully created coinbase exchange client")
	_ = ex.SupportedInterval()
	_ = ex.PlatformFeeCurrency()
}

func TestExchange_Symbols(t *testing.T) {
	globalSymbol := "NOTEXIST"
	localSymbol := toLocalSymbol(globalSymbol)
	assert.Equal(t, globalSymbol, toGlobalSymbol(localSymbol))
	assert.Equal(t, localSymbol, toLocalSymbol(globalSymbol))

	globalSymbol = "ETHUSD"
	localSymbol = toLocalSymbol(globalSymbol)
	assert.Equal(t, globalSymbol, toGlobalSymbol(localSymbol))
	assert.Equal(t, localSymbol, toLocalSymbol(globalSymbol))
}

func TestExchange_OrdersAPI(t *testing.T) {
	ex, saveRecord := getExchangeOrSkip(t)
	defer saveRecord()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	// should succeed
	symbol := "ETHUSD"
	markets, err := ex.QueryMarkets(ctx)
	assert.NoError(t, err)
	market, ok := markets[symbol]
	assert.True(t, ok)
	order, err := ex.SubmitOrder(
		ctx,
		types.SubmitOrder{
			Market:   market,
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
	assert.NotNil(t, order)

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

func TestExchange_CancelOrdersBySymbol(t *testing.T) {
	ex, saveRecord := getExchangeOrSkip(t)
	defer saveRecord()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	// test cancel order by symbol
	symbol := "ETHUSD"
	markets, err := ex.QueryMarkets(ctx)
	assert.NoError(t, err)
	market, ok := markets[symbol]
	assert.True(t, ok)
	order, err := ex.SubmitOrder(
		ctx,
		types.SubmitOrder{
			Market:   market,
			Side:     types.SideTypeBuy,
			Type:     types.OrderTypeLimit,
			Price:    fixedpoint.MustNewFromString("0.01"),
			Quantity: fixedpoint.MustNewFromString("100"), // min funds is $1
		})
	assert.NoError(t, err)
	for i := 0; i < 5; i++ {
		if order.OriginalStatus == "open" {
			break
		}
		time.Sleep(time.Millisecond * 500)
		order, err = ex.QueryOrder(ctx, order.AsQuery())
		assert.NoError(t, err)
	}
	_, err = ex.CancelOrdersBySymbol(ctx, symbol)
	assert.NoError(t, err)
}

func TestExchange_QueryAccount(t *testing.T) {
	ex, saveRecord := getExchangeOrSkip(t)
	defer saveRecord()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	_, err := ex.QueryAccount(ctx)
	assert.NoError(t, err)
}

func TestExchange_QueryAccountBalances(t *testing.T) {
	ex, saveRecord := getExchangeOrSkip(t)
	defer saveRecord()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	_, err := ex.QueryAccountBalances(ctx)
	assert.NoError(t, err)
}

func TestExchange_QueryOpenOrders(t *testing.T) {
	ex, saveRecord := getExchangeOrSkip(t)
	defer saveRecord()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	symbols := []string{"BTCUSD", "ETHUSD", "ETHBTC"}
	for _, k := range symbols {
		_, err := ex.QueryOpenOrders(ctx, k)
		assert.NoError(t, err)
	}
}

func TestExchange_QueryMarkets(t *testing.T) {
	ex, saveRecord := getExchangeOrSkip(t)
	defer saveRecord()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	_, err := ex.QueryMarkets(ctx)
	assert.NoError(t, err)
}

func TestExchange_QueryTicker(t *testing.T) {
	ex, saveRecord := getExchangeOrSkip(t)
	defer saveRecord()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	ticker, err := ex.QueryTicker(ctx, "BTCUSD")
	assert.NoError(t, err)
	assert.NotNil(t, ticker)
}

func TestExchange_QueryTickers(t *testing.T) {
	ex, saveRecord := getExchangeOrSkip(t)
	defer saveRecord()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	symbols := []string{"BTCUSD", "ETHUSD", "ETHBTC"}
	tickers, err := ex.QueryTickers(ctx, symbols...)
	assert.NoError(t, err)
	assert.NotNil(t, tickers)
}

func TestExchange_QueryKLines(t *testing.T) {
	ex, saveRecord := getExchangeOrSkip(t)
	defer saveRecord()

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

func TestExchange_QueryOrderTrades(t *testing.T) {
	ex, saveRecord := getExchangeOrSkip(t)
	defer saveRecord()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	trades, err := ex.QueryOrderTrades(ctx, types.OrderQuery{Symbol: "ETHUSDT"})
	assert.NoError(t, err)
	assert.NotNil(t, trades)
}

func TestExchange_QueryTrades(t *testing.T) {
	ex, saveRecord := getExchangeOrSkip(t)
	defer saveRecord()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	symbol := "BTCUSDT"
	trades, err := ex.QueryTrades(ctx, symbol, &types.TradeQueryOptions{})
	assert.NoError(t, err)
	assert.NotNil(t, trades)
	assert.Greater(t, len(trades), 0)

	// query the last 100 trades
	trades, err = ex.QueryTrades(ctx, symbol, &types.TradeQueryOptions{
		Limit: 20,
	})
	assert.NoError(t, err)
	assert.NotNil(t, trades)
	assert.Equal(t, len(trades), 20)

	// query the last 10 trades with last trade ID
	lastTradeID := trades[10].ID
	trades, err = ex.QueryTrades(ctx, symbol, &types.TradeQueryOptions{
		LastTradeID: lastTradeID,
	})
	assert.NoError(t, err)
	assert.NotNil(t, trades)
	assert.Equal(t, len(trades), 10)
}

func TestExchange_QueryClosedOrders(t *testing.T) {
	ex, saveRecord := getExchangeOrSkip(t)
	defer saveRecord()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	symbol := "BTCUSDT"
	startTime, _ := time.Parse(
		time.RFC3339,
		"2025-06-15T03:20:00Z",
	)
	endTime, _ := time.Parse(
		time.RFC3339,
		"2025-06-20T06:11:10Z",
	)
	orders, err := ex.QueryClosedOrders(ctx, symbol, startTime, endTime, 0)
	assert.NoError(t, err)
	assert.NotNil(t, orders)
	assert.Greater(t, len(orders), 0)

	// startTime and endTime are inclusive
	startTime = orders[15].CreationTime.Time()
	endTime = orders[10].CreationTime.Time()
	orders, err = ex.QueryClosedOrders(ctx, symbol, startTime, endTime, 0)
	assert.NoError(t, err)
	assert.NotNil(t, orders)
	assert.Equal(t, len(orders), 6)
}

func TestExchange_QueryDepositHistory(t *testing.T) {
	ex, _ := getExchangeOrSkip(t)
	if ex.apiKey == "" {
		t.Skip("skip test for CI")
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	since, _ := time.Parse(
		"2006-01-02",
		"2025-02-01",
	)
	until, _ := time.Parse(
		"2006-01-02",
		"2025-07-08",
	)
	asset := "USDC"
	deposits, err := ex.QueryDepositHistory(ctx, asset, since, until)
	assert.NoError(t, err)
	assert.NotNil(t, deposits)
	assert.Greater(t, len(deposits), 0)

	deposits, err = ex.QueryDepositHistory(ctx, "", since, until)
	assert.NoError(t, err)
	assert.NotNil(t, deposits)
	assert.Greater(t, len(deposits), 0)
}

func TestExchange_QueryWithdrawHistory(t *testing.T) {
	ex, _ := getExchangeOrSkip(t)
	if ex.apiKey == "" {
		t.Skip("skip test for CI")
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	since, _ := time.Parse(
		"2006-01-02",
		"2025-02-01",
	)
	until, _ := time.Parse(
		"2006-01-02",
		"2025-07-08",
	)
	asset := "USDC"
	withdraws, err := ex.QueryWithdrawHistory(ctx, asset, since, until)
	assert.NoError(t, err)
	assert.NotNil(t, withdraws)
	assert.Greater(t, len(withdraws), 0)

	withdraws, err = ex.QueryWithdrawHistory(ctx, "", since, until)
	assert.NoError(t, err)
	assert.NotNil(t, withdraws)
	assert.Greater(t, len(withdraws), 0)
}

func getExchangeOrSkip(t *testing.T) (*Exchange, func()) {
	key, secret, passphrase, ok := testutil.IntegrationTestWithPassphraseConfigured(t, "COINBASE")
	ex := New(key, secret, passphrase, 0)
	isRecording, saveRecord := httptesting.RunHttpTestWithRecorder(t, ex.client.HttpClient, "testdata/"+t.Name()+".json")

	if isRecording && !ok {
		t.Skipf("api keys are not configured, skip test: %s", t.Name())
		return nil, nil
	}

	return ex, saveRecord
}
