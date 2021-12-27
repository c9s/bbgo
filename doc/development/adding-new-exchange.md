# Adding New Exchange

Open an issue and paste the following checklist to that issue.

You should send multiple small pull request to implement them.

**Please avoid sending a pull request with huge changes**

## Checklist

Exchange Interface - minimum requirement for trading

- [ ] QueryMarkets
- [ ] QueryTickers
- [ ] QueryOpenOrders
- [ ] SubmitOrders
- [ ] CancelOrders
- [ ] NewStream

Trading History Service Interface - used for syncing user trading data

- [ ] QueryClosedOrders
- [ ] QueryTrades

Back-testing service - kline data is used for back-testing

- [ ] QueryKLines

Convert functions:

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
  - [ ] KLine message parser (required for backtesting)
  - [ ] Public trade message parser (optional)
  - [ ] Ticker message parser (optional)
- [ ] ping/pong handling.
- [ ] heart-beat hanlding or keep-alive handling.
- [ ] handling reconnect

Database

- [ ] Add a new kline table for the exchange (this is required for back-testing)
  - [ ] Add MySQL migration SQL
  - [ ] Add SQLite migration SQL

Exchange Factory

- [ ] Add the exchange constructor to the exchange instance factory function.
- [ ] Add extended fields to the ExchangeSession struct. (optional)

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

## Testing order book stream

```shell
godotenv -f .env.local -- go run ./cmd/bbgo orderbook --config config/bbgo.yaml --session kucoin --symbol BTCUSDT
```

## Testing user data stream

```shell
godotenv -f .env.local -- go run ./cmd/bbgo --config config/bbgo.yaml userdatastream --session kucoin
```

### Testing order submit

```shell
godotenv -f .env.local -- go run ./cmd/bbgo submit-order --session=kucoin --symbol=BTCUSDT --side=buy --price=18000 --quantity=0.001
```

### Testing open orders query

```shell
godotenv -f .env.local -- go run ./cmd/bbgo list-orders --session kucoin --symbol=BTCUSDT open
godotenv -f .env.local -- go run ./cmd/bbgo list-orders --session kucoin --symbol=BTCUSDT closed
```

### Testing order cancel

```shell
godotenv -f .env.local -- go run ./cmd/bbgo cancel-order --session kucoin --order-uuid 61c745c44592c200014abdcf
```
