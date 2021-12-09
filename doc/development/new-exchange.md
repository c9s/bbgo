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

Stream

- [ ] UserDataStream
  - [ ] Trade message parser
  - [ ] Order message parser
  - [ ] Account message parser
  - [ ] Balance message parser
- [ ] MarketDataStream
  - [ ] OrderBook message parser (or depth)
  - [ ] KLine message parser
  - [ ] Public trade message parser
- [ ] ping/pong
- [ ] heart beat integration
- [ ] handling reconnect

Exchange Factory

- [ ] Add the exchange constructor to the exchange instance factory function.
- [ ] Add extended fields to the ExchangeSession struct. (optional)
