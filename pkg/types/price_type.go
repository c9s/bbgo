package types

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
)

type PriceType string

const (
	PriceTypeLast  PriceType = "LAST"
	PriceTypeBuy   PriceType = "BUY"  // BID
	PriceTypeSell  PriceType = "SELL" // ASK
	PriceTypeMid   PriceType = "MID"
	PriceTypeMaker PriceType = "MAKER"
	PriceTypeTaker PriceType = "TAKER"
)

func (p PriceType) Map(ticker *Ticker, side SideType) fixedpoint.Value {
	price := ticker.Last

	switch p {
	case PriceTypeLast:
		price = ticker.Last
	case PriceTypeBuy:
		price = ticker.Buy
	case PriceTypeSell:
		price = ticker.Sell
	case PriceTypeMid:
		price = ticker.Buy.Add(ticker.Sell).Div(fixedpoint.NewFromInt(2))
	case PriceTypeMaker:
		if side == SideTypeBuy {
			price = ticker.Buy
		} else if side == SideTypeSell {
			price = ticker.Sell
		}
	case PriceTypeTaker:
		if side == SideTypeBuy {
			price = ticker.Sell
		} else if side == SideTypeSell {
			price = ticker.Buy
		}
	}

	return price
}
