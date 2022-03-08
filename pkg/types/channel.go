package types

type Channel string

var BookChannel = Channel("book")
var KLineChannel = Channel("kline")
var HeikinAshiChannel = Channel("heikinashi")
var BookTickerChannel = Channel("bookticker")
