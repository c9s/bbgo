# Adding New Exchange

Open an issue and paste the following checklist to that issue.

You should send multiple small pull request to implement them.

**Please avoid sending a pull request with huge changes**

## Checklist

Exchange Interface (minimum)

- [ ] QueryMarkets
- [ ] QueryKLines
- [ ] QueryTickers
- [ ] QueryOrders
- [ ] QueryTrades
- [ ] SubmitOrders

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


## Testing user data stream

```shell
go run ./cmd/bbgo   --config config/bbgo.yaml userdatastream --session kucoin
```