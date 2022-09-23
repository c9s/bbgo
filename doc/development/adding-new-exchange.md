# Adding New Exchange

Open an issue and paste the following checklist to that issue.

You should send multiple small pull request to implement them.

**Please avoid sending a pull request with huge changes**

**Important** -- for the underlying http API please use `requestgen` <https://github.com/c9s/requestgen> to generate the
requests.

## Checklist

Exchange Interface - (required) the minimum requirement for spot trading

- [ ] QueryMarkets
- [ ] QueryTickers
- [ ] QueryOpenOrders
- [ ] SubmitOrders
- [ ] CancelOrders
- [ ] NewStream

Trading History Service Interface - (optional) used for syncing user trading data

- [ ] QueryClosedOrders
- [ ] QueryTrades

Order Query Service Interface - (optional) used for querying order status

- [ ] QueryOrder

Back-testing service - (optional, required by backtesting) kline data is used for back-testing

- [ ] QueryKLines

Convert functions (required):

- [ ] MarketData convert functions
    - [ ] toGlobalMarket
    - [ ] toGlobalTicker
    - [ ] toGlobalKLine
- [ ] UserData convert functions
    - [ ] toGlobalOrder
    - [ ] toGlobalTrade
    - [ ] toGlobalAccount
    - [ ] toGlobalBalance

Stream

- [ ] UserDataStream
    - [ ] Trade message parser
    - [ ] Order message parser
    - [ ] Account message parser
    - [ ] Balance message parser
- [ ] MarketDataStream
    - [ ] OrderBook message parser (or depth)
    - [ ] KLine message parser (required for backtesting and strategy)
    - [ ] Public trade message parser (optional)
    - [ ] Ticker message parser (optional)
- [ ] ping/pong handling. (you can reuse the existing types.StandardStream)
- [ ] heart-beat hanlding or keep-alive handling. (already included in types.StandardStream)
- [ ] handling reconnect. (already included in types.StandardStream)

Database

- [ ] Add a new kline table for the exchange (required for back-testing)
    - [ ] Add MySQL migration SQL
    - [ ] Add SQLite migration SQL

Exchange Factory

- [ ] Add the exchange constructor to the exchange instance factory function.
- [ ] Add extended fields to the ExchangeSession struct. (optional)

# Tools

- Use a tool to convert JSON response to Go struct <https://mholt.github.io/json-to-go/>
- Use requestgen to generate request builders <https://github.com/c9s/requestgen>
- Use callbackgen to generate callbacks <https://github.com/c9s/callbackgen>

# Implementation

Go to `pkg/types/exchange.go` and add your exchange type:

```
const (
	ExchangeMax      = ExchangeName("max")
	ExchangeBinance  = ExchangeName("binance")
	ExchangeFTX      = ExchangeName("ftx")
	ExchangeOKEx     = ExchangeName("okex")
        ExchangeKucoin   = ExchangeName("kucoin")
        ExchangeBacktest = ExchangeName("backtest")
)
```

Go to `pkg/cmd/cmdutil/exchange.go` and add your exchange to the factory

```
func NewExchangeStandard(n types.ExchangeName, key, secret, passphrase, subAccount string) (types.Exchange, error) {
	switch n {

	case types.ExchangeFTX:
		return ftx.NewExchange(key, secret, subAccount), nil

	case types.ExchangeBinance:
		return binance.New(key, secret), nil

	case types.ExchangeOKEx:
		return okex.New(key, secret, passphrase), nil

    // ...
	}
}
```

## Using requestgen

### Alias

You can put the go:generate alias on the top of the file:

```
//go:generate -command GetRequest requestgen -method GET
//go:generate -command PostRequest requestgen -method POST
//go:generate -command DeleteRequest requestgen -method DELETE
```

Please note that the alias only works in the same file.

### Defining Request Type Names

Please define request type name in the following format:

```
{Verb}{Service}{Resource}Request
```

for example:

```
type GetMarginMarketsRequest struct {
	client requestgen.APIClient
}
```

then you can attach the go:generate command on that type:

```

//go:generate GetRequest -url "/api/v3/wallet/m/limits" -type GetMarginBorrowingLimitsRequest -responseType .MarginBorrowingLimitMap
```

## Un-marshalling Timestamps

For millisecond timestamps, you can use `types.MillisecondTimestamp`, it will automatically convert the timestamp into
time.Time:

```
type MarginInterestRecord struct {
  Currency     string                     `json:"currency"`
  CreatedAt    types.MillisecondTimestamp `json:"created_at"`
}
```

## Un-marshalling numbers

For number fields, especially floating numbers, please use `fixedpoint.Value`, it can parse int, float64, float64 in
string:

```
type A struct {
  Amount       fixedpoint.Value           `json:"amount"`
}
```

## Test Market Data Stream

### Test order book stream

```shell
godotenv -f .env.local -- go run ./cmd/bbgo orderbook --config config/bbgo.yaml --session kucoin --symbol BTCUSDT
```

## Test User Data Stream

```shell
godotenv -f .env.local -- go run ./cmd/bbgo --config config/bbgo.yaml userdatastream --session kucoin
```

## Test Restful Endpoints

You can choose the session name to set-up for testing:

```shell
export BBGO_SESSION=ftx
export BBGO_SESSION=kucoin
export BBGO_SESSION=binance
```

### Test user account balance

```shell
godotenv -f .env.local -- go run ./cmd/bbgo balances --session $BBGO_SESSION
```

### Test order submit

```shell
godotenv -f .env.local -- go run ./cmd/bbgo submit-order --session $BBGO_SESSION --symbol=BTCUSDT --side=buy --price=18000 --quantity=0.001
```

### Test open orders query

```shell
godotenv -f .env.local -- go run ./cmd/bbgo list-orders --session $BBGO_SESSION --symbol=BTCUSDT open
godotenv -f .env.local -- go run ./cmd/bbgo list-orders --session $BBGO_SESSION --symbol=BTCUSDT closed
```

### Test order status

```shell
godotenv -f .env.local -- go run ./cmd/bbgo get-order --session $BBGO_SESSION --order-id ORDER_ID
```

### Test order cancel

```shell
godotenv -f .env.local -- go run ./cmd/bbgo cancel-order --session $BBGO_SESSION --order-uuid 61c745c44592c200014abdcf
```
