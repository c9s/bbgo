package types

type Channel string

var BookChannel = Channel("book")
var KLineChannel = Channel("kline")
var BookTickerChannel = Channel("bookticker")
var MarketTradeChannel = Channel("trade")
