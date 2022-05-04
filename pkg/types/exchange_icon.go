package types

func ExchangeFooterIcon(exName ExchangeName) string {
	footerIcon := ""

	switch exName {
	case ExchangeBinance:
		footerIcon = "https://bin.bnbstatic.com/static/images/common/favicon.ico"
	case ExchangeMax:
		footerIcon = "https://max.maicoin.com/favicon-16x16.png"
	case ExchangeFTX:
		footerIcon = "https://ftx.com/favicon.ico?v=2"
	case ExchangeOKEx:
		footerIcon = "https://static.okex.com/cdn/assets/imgs/MjAxODg/D91A7323087D31A588E0D2A379DD7747.png"
	case ExchangeKucoin:
		footerIcon = "https://assets.staticimg.com/cms/media/7AV75b9jzr9S8H3eNuOuoqj8PwdUjaDQGKGczGqTS.png"
	}

	return footerIcon
}
