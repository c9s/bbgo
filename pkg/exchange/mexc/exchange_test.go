package mexc

import (
	"context"
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func getExchange(t *testing.T) *Exchange {
	key := os.Getenv("MEXC_API_KEY")
	secret := os.Getenv("MEXC_API_SECRET")
	if len(key) == 0 || len(secret) == 0 {
		t.Skip("api key/secret not configured")
	}
	return &Exchange{key, secret, nil}
}

func Test_Ping(t *testing.T) {
	ex := getExchange(t)
	assert.True(t, ex.ping(context.Background()))
}

func Test_Time(t *testing.T) {
	ex := getExchange(t)
	timestamp, err := ex.time(context.Background())
	assert.Equal(t, err, nil)
	assert.InDelta(t, timestamp, time.Now().UnixMilli(), 60000)
}

func Test_Ticker(t *testing.T) {
	ex := getExchange(t)
	ticker, err := ex.QueryTicker(context.Background(), "APEUSDT")
	assert.Equal(t, err, nil)
	assert.True(t, ticker.High.Compare(ticker.Low) > 0)
	assert.InDelta(t, ticker.Time.UnixMilli(), time.Now().UnixMilli(), 86400_000)
}

func Test_Tickers(t *testing.T) {
	ex := getExchange(t)
	tickers, err := ex.QueryTickers(context.Background(), "APEUSDT", "ETHUSDT")
	assert.Equal(t, err, nil)
	assert.Equal(t, len(tickers), 2)
	assert.InDelta(t, tickers["APEUSDT"].Time.UnixMilli(), time.Now().UnixMilli(), 86400_000)
	assert.InDelta(t, tickers["ETHUSDT"].Time.UnixMilli(), time.Now().UnixMilli(), 86400_000)

	tickers, err = ex.QueryTickers(context.Background())
	assert.Equal(t, err, nil)
	assert.Greater(t, len(tickers), 100) // 1980 symbols @ 19/5/2022
}

func Test_Markets(t *testing.T) {
	ex := getExchange(t)
	markets, err := ex.QueryMarkets(context.Background())
	assert.Equal(t, err, nil)
	assert.Greater(t, len(markets), 0)
	assert.Equal(t, markets["MXUSDT"].PricePrecision, 4)
	assert.Equal(t, markets["MXUSDT"].VolumePrecision, 2)
}

func Test_MarketOrders(t *testing.T) {
	ex := getExchange(t)
	orders, err := ex.QueryMarketOrders(context.Background(), "MXUSDT")
	assert.Equal(t, err, nil)
	assert.Equal(t, len(orders), 200)
}

func Test_KLines(t *testing.T) {
	ex := getExchange(t)
	klines, err := ex.QueryKLines(context.Background(), "MXUSDT", types.Interval5m,
		types.KLineQueryOptions{})
	assert.Equal(t, err, nil)
	assert.Equal(t, len(klines), 500)
}

func Test_QueryOrder(t *testing.T) {
	ex := getExchange(t)
	order, err := ex.QueryOrder(context.Background(), types.OrderQuery{Symbol: "USTUSDT", OrderID: "156198093501521920"})
	t.Skip("private api skip")
	assert.Equal(t, err, nil)
	assert.NotEqual(t, order, nil)
}

func Test_QueryOpenOrders(t *testing.T) {
	ex := getExchange(t)
	orders, err := ex.QueryOpenOrders(context.Background(), "USTUSDT")
	t.Skip("private api skip")
	assert.Equal(t, err, nil)
	assert.Greater(t, len(orders), 0)
}

func Test_QueryAccount(t *testing.T) {
	ex := getExchange(t)
	account, err := ex.QueryAccount(context.Background())
	assert.Equal(t, err, nil)
	assert.Greater(t, account.TakerFeeRate, fixedpoint.Zero)
}
