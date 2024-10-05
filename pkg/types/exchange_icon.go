package types

func ExchangeFooterIcon(exName ExchangeName) string {
	footerIcon := ""

	switch exName {
	case ExchangeBinance:
		footerIcon = "https://bin.bnbstatic.com/static/images/common/favicon.ico"
	case ExchangeMax:
		footerIcon = "https://max.maicoin.com/favicon-16x16.png"
	case ExchangeOKEx:
		footerIcon = "https://www.okx.com/cdn/assets/plugins/2022/01/07104210.png"
	case ExchangeKucoin:
		footerIcon = "https://assets.staticimg.com/cms/media/7AV75b9jzr9S8H3eNuOuoqj8PwdUjaDQGKGczGqTS.png"
	}

	return footerIcon
}
