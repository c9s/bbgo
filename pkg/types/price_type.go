package types

import (
	"encoding/json"
	"strings"

	"github.com/pkg/errors"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

type PriceType string

const (
	// PriceTypeLast uses the last price from the given ticker
	PriceTypeLast PriceType = "LAST"

	// PriceTypeBid uses the bid price from the given ticker
	PriceTypeBid PriceType = "BID"

	// PriceTypeAsk uses the ask price from the given ticker
	PriceTypeAsk PriceType = "ASK"

	// PriceTypeMid calculates the middle price from the given ticker
	PriceTypeMid PriceType = "MID"

	PriceTypeMaker PriceType = "MAKER"
	PriceTypeTaker PriceType = "TAKER"

	// See best bid offer types
	// https://www.binance.com/en/support/faq/understanding-and-using-bbo-orders-on-binance-futures-7f93c89ef09042678cfa73e8a28612e8

	PriceTypeBestBidOfferCounterParty1 PriceType = "COUNTERPARTY1"
	PriceTypeBestBidOfferCounterParty5 PriceType = "COUNTERPARTY5"

	PriceTypeBestBidOfferQueue1 PriceType = "QUEUE1"
	PriceTypeBestBidOfferQueue5 PriceType = "QUEUE5"
)

var ErrInvalidPriceType = errors.New("invalid price type")

func ParsePriceType(s string) (p PriceType, err error) {
	p = PriceType(strings.ToUpper(s))
	switch p {
	case PriceTypeLast, PriceTypeBid, PriceTypeAsk,
		PriceTypeMid, PriceTypeMaker, PriceTypeTaker,
		PriceTypeBestBidOfferCounterParty1, PriceTypeBestBidOfferCounterParty5,
		PriceTypeBestBidOfferQueue1, PriceTypeBestBidOfferQueue5:
		return p, err
	}
	return p, ErrInvalidPriceType
}

func (p *PriceType) UnmarshalJSON(data []byte) error {
	var s string

	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	t, err := ParsePriceType(s)
	if err != nil {
		return err
	}

	*p = t

	return nil
}

// GetPrice returns the price from the given ticker based on the price type
func (p PriceType) GetPrice(ticker *Ticker, side SideType) fixedpoint.Value {
	return ticker.GetPrice(side, p)
}
