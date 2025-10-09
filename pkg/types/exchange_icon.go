package types

func ExchangeFooterIcon(exName ExchangeName) string {
	footerIcon := ""

	switch exName {
	case ExchangeBinance:
		footerIcon = "https://bin.bnbstatic.com/static/images/common/favicon.ico"
	case ExchangeMax:
		footerIcon = "https://max.maicoin.com/favicon-16x16.png"
	case ExchangeOKEx:
		footerIcon = "https://static.okex.com/cdn/assets/imgs/MjAxODg/D91A7323087D31A588E0D2A379DD7747.png"
	case ExchangeKucoin:
		footerIcon = "https://assets.staticimg.com/cms/media/7AV75b9jzr9S8H3eNuOuoqj8PwdUjaDQGKGczGqTS.png"

	case ExchangeCoinBase:
		// footerIcon = "https://www.coinbase.com/favicon.ico"
		// footerIcon = "https://www.coinbase.com/assets/sw-cache/a_6z3OfXe9.png" // 32x32
		footerIcon = "https://www.coinbase.com/assets/sw-cache/a_DVA0h2KN.png" // 96x96

	case ExchangeBitfinex:
		footerIcon = "https://www.bitfinex.com/assets/favicons/bitfinex.ico"
	}

	return footerIcon
}
