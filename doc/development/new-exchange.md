# Adding New Exchange

Open an issue and paste the following checklist to that issue. You should send multiple small pull request to implement them.

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
- [ ] ping/pong
- [ ] heart beat integration
- [ ] handling reconnect

Database

- [ ] Add a new kline table for the exchange (this is required for back-testing)
  - [ ] Add MySQL migration SQL
  - [ ] Add SQLite migration SQL

Exchange Factory

- [ ] Add the exchange constructor to the exchange instance factory function.
- [ ] Add extended fields to the ExchangeSession struct. (optional)
