package types

import (
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

type Ticker struct {
	Time   time.Time
	Volume fixedpoint.Value // `volume` from Max & binance
	Last   fixedpoint.Value // `last` from Max, `lastPrice` from binance
	Open   fixedpoint.Value // `open` from Max, `openPrice` from binance
	High   fixedpoint.Value // `high` from Max, `highPrice` from binance
	Low    fixedpoint.Value // `low` from Max, `lowPrice` from binance
	Buy    fixedpoint.Value // `buy` from Max, `bidPrice` from binance
	Sell   fixedpoint.Value // `sell` from Max, `askPrice` from binance
}

// GetValidPrice returns the valid price from the ticker
// if the last price is not zero, return the last price
// if the buy price is not zero, return the buy price
// if the sell price is not zero, return the sell price
// otherwise return the open price
func (t *Ticker) GetValidPrice() fixedpoint.Value {
	if !t.Last.IsZero() {
		return t.Last
	}

	if !t.Buy.IsZero() {
		return t.Buy
	}

	if !t.Sell.IsZero() {
		return t.Sell
	}

	return t.Open
}

func (t *Ticker) String() string {
	return fmt.Sprintf("O:%s H:%s L:%s LAST:%s BID/ASK:%s/%s TIME:%s", t.Open, t.High, t.Low, t.Last, t.Buy, t.Sell, t.Time.String())
}

func (t *Ticker) GetPrice(side SideType, p PriceType) fixedpoint.Value {
	switch p {
	case PriceTypeBestBidOfferQueue5, PriceTypeBestBidOfferCounterParty5:
		log.Warnf("price type %s is not supported with ticker", p)
	}

	price := t.Last

	switch p {
	case PriceTypeLast:
		price = t.Last
	case PriceTypeBid:
		price = t.Buy
	case PriceTypeAsk:
		price = t.Sell
	case PriceTypeMid:
		price = t.Buy.Add(t.Sell).Div(fixedpoint.NewFromInt(2))

	case PriceTypeMaker, PriceTypeBestBidOfferQueue1, PriceTypeBestBidOfferQueue5:
		if side == SideTypeBuy {
			price = t.Buy
		} else if side == SideTypeSell {
			price = t.Sell
		}

	case PriceTypeTaker, PriceTypeBestBidOfferCounterParty1, PriceTypeBestBidOfferCounterParty5:
		if side == SideTypeBuy {
			price = t.Sell
		} else if side == SideTypeSell {
			price = t.Buy
		}
	}

	return price
}

// Validate checks if the Ticker fields are correctly set.
// It returns an error if any required field is not set or invalid.
func (t *Ticker) Validate() error {
	if t == nil {
		return fmt.Errorf("ticker is nil")
	}
	if t.Time.IsZero() {
		return fmt.Errorf("ticker time is zero")
	}
	if t.Last.IsZero() && t.Buy.IsZero() && t.Sell.IsZero() && t.Open.IsZero() {
		return fmt.Errorf("ticker has no valid price")
	}
	if t.High.IsZero() {
		return fmt.Errorf("ticker high price is zero")
	}
	if t.Low.IsZero() {
		return fmt.Errorf("ticker low price is zero")
	}
	if t.Volume.IsZero() {
		return fmt.Errorf("ticker volume is zero")
	}
	return nil
}
