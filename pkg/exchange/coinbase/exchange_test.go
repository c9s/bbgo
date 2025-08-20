package coinbase

import (
	"context"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/testing/httptesting"
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
	t.Logf("created order: %+v", order)

	// test query open orders
	order, err = ex.QueryOrder(ctx, types.OrderQuery{Symbol: symbol, OrderID: order.UUID, ClientOrderID: order.UUID})
	assert.NoError(t, err)
	assert.NotNil(t, order)
	t.Logf("queried order: %+v", order)

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
	t.Logf("queried open orders: %+v", orders)
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
	t.Logf("market: %+v", market)
	assert.NotEmpty(t, market)
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
	t.Logf("created order: %+v", order)

	for i := 0; i < 5; i++ {
		if order.OriginalStatus == "open" {
			break
		}
		time.Sleep(time.Millisecond * 500)
		order, err = ex.QueryOrder(ctx, order.AsQuery())
		assert.NoError(t, err)
	}
	canceledOrders, err := ex.CancelOrdersBySymbol(ctx, symbol)
	t.Logf("canceled orders: %+v", canceledOrders)

	assert.NoError(t, err)
	for _, order := range canceledOrders {
		assert.NotEmpty(t, order)
	}
}

func TestExchange_QueryAccount(t *testing.T) {
	ex, saveRecord := getExchangeOrSkip(t)
	defer saveRecord()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	account, err := ex.QueryAccount(ctx)
	t.Logf("queried account: %+v", account)
	assert.NotEmpty(t, account)
	assert.NoError(t, err)
}

func TestExchange_QueryAccountBalances(t *testing.T) {
	ex, saveRecord := getExchangeOrSkip(t)
	defer saveRecord()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	bm, err := ex.QueryAccountBalances(ctx)
	t.Logf("queried account balances: %+v", bm)
	assert.NotEmpty(t, bm)
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

	mm, err := ex.QueryMarkets(ctx)
	t.Logf("queried markets: %+v", mm)
	for symbol, market := range mm {
		assert.NotEmpty(t, symbol)
		assert.NotEmpty(t, market)
	}

	assert.NoError(t, err)
}

func TestExchange_QueryTicker(t *testing.T) {
	ex, saveRecord := getExchangeOrSkip(t)
	defer saveRecord()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	ticker, err := ex.QueryTicker(ctx, "BTCUSD")
	t.Logf("queried ticker: %+v", ticker)
	assert.NoError(t, err)
	assert.NotNil(t, ticker)
	assert.NotEmpty(t, ticker)
}

func TestExchange_QueryTickers(t *testing.T) {
	ex, saveRecord := getExchangeOrSkip(t)
	defer saveRecord()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	symbols := []string{"BTCUSD", "ETHUSD", "ETHBTC"}
	tickers, err := ex.QueryTickers(ctx, symbols...)
	t.Logf("queried tickers: %+v", tickers)
	assert.NoError(t, err)
	assert.NotNil(t, tickers)
	for symbol, ticker := range tickers {
		assert.NotEmpty(t, symbol)
		assert.NotEmpty(t, ticker)
	}
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
	t.Logf("queried 1m klines: %+v", klines)
	assert.NoError(t, err)
	assert.NotNil(t, klines)
	for _, kline := range klines {
		assert.NotEmpty(t, kline)
	}

	endTime := time.Now()
	startTime := endTime.Add(-time.Minute * 5)
	klines, err = ex.QueryKLines(
		ctx,
		"BTCUSD",
		types.Interval1m,
		types.KLineQueryOptions{
			StartTime: &startTime,
			EndTime:   &endTime,
		},
	)
	t.Logf("queried 1m klines (%s~%s): %+v", startTime, endTime, klines)
	assert.NoError(t, err)
	assert.NotNil(t, klines)
	for _, kline := range klines {
		assert.NotEmpty(t, kline.Symbol)
	}
}

func TestExchange_QueryOrderTrades(t *testing.T) {
	ex, saveRecord := getExchangeOrSkip(t)
	defer saveRecord()

	ctx := context.Background()

	trades, err := ex.QueryOrderTrades(ctx, types.OrderQuery{Symbol: "BTCUSDT"})
	t.Logf("queried trades: %+v", trades)
	assert.NoError(t, err)
	assert.NotNil(t, trades)
	assert.Greater(t, len(trades), 0)
	for _, trade := range trades {
		assert.NotEmpty(t, trade)
	}
}

func TestExchange_QueryTrades(t *testing.T) {
	ex, saveRecord := getExchangeOrSkip(t)
	defer saveRecord()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	symbol := "BTCUSDT"
	trades, err := ex.QueryTrades(ctx, symbol, &types.TradeQueryOptions{})
	t.Logf("queried trades: %+v", trades)
	assert.NoError(t, err)
	assert.NotNil(t, trades)
	assert.Greater(t, len(trades), 0)
	for _, trade := range trades {
		assert.NotEmpty(t, trade)
	}

	// query the last 20 trades
	trades, err = ex.QueryTrades(ctx, symbol, &types.TradeQueryOptions{
		Limit: 20,
	})
	t.Logf("queried last 20 trades: %+v", trades)
	assert.NoError(t, err)
	assert.NotNil(t, trades)
	assert.Equal(t, len(trades), 20)
	for _, trade := range trades {
		assert.NotEmpty(t, trade)
	}

	// query the last 10 trades with last trade ID
	lastTradeID := trades[10].ID
	trades, err = ex.QueryTrades(ctx, symbol, &types.TradeQueryOptions{
		LastTradeID: lastTradeID,
	})
	t.Logf("queried trades after last trade ID %d: %+v", lastTradeID, trades)
	assert.NoError(t, err)
	assert.NotNil(t, trades)
	assert.Equal(t, len(trades), 10)
	for _, trade := range trades {
		assert.NotEmpty(t, trade)
	}
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
	t.Logf("queried closed orders (%s~%s): %+v", startTime, endTime, orders)
	assert.NoError(t, err)
	assert.NotNil(t, orders)
	assert.Greater(t, len(orders), 0)
	for _, order := range orders {
		assert.NotEmpty(t, order)
	}

	// startTime and endTime are inclusive
	startTime = orders[15].CreationTime.Time()
	endTime = orders[10].CreationTime.Time()
	orders, err = ex.QueryClosedOrders(ctx, symbol, startTime, endTime, 0)
	t.Logf("queried closed orders (%s~%s): %+v", startTime, endTime, orders)
	assert.NoError(t, err)
	assert.NotNil(t, orders)
	assert.Equal(t, len(orders), 6)
	for _, order := range orders {
		assert.NotEmpty(t, order)
	}
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

func TestExchange_queryAccountIDsBySymbols(t *testing.T) {
	symbols := []string{"BTCUSD", "BTCUSDT", "ETHUSD"}
	ex, saveRecord := getExchangeOrSkip(t)
	defer saveRecord()

	accountIDs, err := ex.queryAccountIDsBySymbols(context.Background(), symbols)
	sort.Slice(accountIDs, func(i, j int) bool {
		return accountIDs[i] < accountIDs[j]
	})
	assert.NoError(t, err)
	assert.NotNil(t, accountIDs)
	assert.Equal(t, []string{"<BTC_ACCOUNT_ID>", "<ETH_ACCOUNT_ID>", "<USDT_ACCOUNT_ID>", "<USD_ACCOUNT_ID>"}, accountIDs)
}

func getExchangeOrSkip(t *testing.T) (*Exchange, func()) {
	key, secret, passphrase, ok := IntegrationTestWithPassphraseConfigured(t, "COINBASE")
	ex := New(key, secret, passphrase, 0)
	isRecording, saveRecord := httptesting.RunHttpTestWithRecorder(t, ex.client.HttpClient, "testdata/"+t.Name()+".json")

	if isRecording && !ok {
		t.Skipf("api keys are not configured, skip test: %s", t.Name())
		return nil, nil
	}

	return ex, saveRecord
}

// copied from testutil, or tests for other exchanges will fail
func IntegrationTestWithPassphraseConfigured(t *testing.T, prefix string) (key, secret, passphrase string, ok bool) {
	var hasKey, hasSecret, hasPassphrase bool

	prefix = strings.ToUpper(prefix)
	key, hasKey = os.LookupEnv(prefix + "_API_KEY")
	secret, hasSecret = os.LookupEnv(prefix + "_API_SECRET")
	passphrase, hasPassphrase = os.LookupEnv(prefix + "_API_PASSPHRASE")
	ok = hasKey && hasSecret && hasPassphrase && os.Getenv("TEST_"+prefix) == "1"
	if ok {
		t.Logf(prefix+" api integration test enabled, key = %s, secret = %s, passphrase= %s", maskSecret(key), maskSecret(secret), maskSecret(passphrase))
	}
	return key, secret, passphrase, ok
}

func maskSecret(s string) string {
	re := regexp.MustCompile(`\b(\w{4})\w+\b`)
	s = re.ReplaceAllString(s, "$1******")
	return s
}
