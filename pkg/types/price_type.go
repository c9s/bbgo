package types

import (
	"encoding/json"
	"strings"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/pkg/errors"
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

var ErrInvalidPriceType = errors.New("invalid price type")

func StrToPriceType(s string) (price PriceType, err error) {
	switch strings.ToLower(s) {
	case "last":
		price = PriceTypeLast
	case "buy":
		price = PriceTypeBuy
	case "sell":
		price = PriceTypeSell
	case "mid":
		price = PriceTypeMid
	case "maker":
		price = PriceTypeMaker
	case "taker":
		price = PriceTypeTaker
	default:
		err = ErrInvalidPriceType
	}
	return price, err
}

func (p *PriceType) UnmarshalJSON(data []byte) error {
	var s string

	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	t, err := StrToPriceType(s)
	if err != nil {
		return err
	}

	*p = t
	return nil
}

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
