package types

type Channel string

const (
	BookChannel        = Channel("book")
	KLineChannel       = Channel("kline")
	BookTickerChannel  = Channel("bookTicker")
	MarketTradeChannel = Channel("trade")
	AggTradeChannel    = Channel("aggTrade")
	ForceOrderChannel  = Channel("forceOrder")

	// channels for futures
	MarkPriceChannel = Channel("markPrice")

	LiquidationOrderChannel = Channel("liquidationOrder")

	// ContractInfoChannel is the contract info provided by the exchange
	ContractInfoChannel = Channel("contractInfo")
)
