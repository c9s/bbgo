package binance

import "github.com/adshao/go-binance/v2"

func BuyerOrSellerLabel(trade *binance.TradeV3) (o string) {
	if trade.IsBuyer {
		o = "BUYER"
	} else {
		o = "SELLER"
	}
	return o
}

func MakerOrTakerLabel(trade *binance.TradeV3) (o string) {
	if trade.IsMaker {
		o += "MAKER"
	} else {
		o += "TAKER"
	}

	return o
}
