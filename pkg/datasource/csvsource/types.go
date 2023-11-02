package csvsource

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type CsvTick struct {
	Exchange        types.ExchangeName         `json:"exchange"`
	TradeID         uint64                     `json:"tradeID"`
	Symbol          string                     `json:"symbol"`
	TickDirection   string                     `json:"tickDirection"`
	Side            types.SideType             `json:"side"`
	Size            fixedpoint.Value           `json:"size"`
	Price           fixedpoint.Value           `json:"price"`
	HomeNotional    fixedpoint.Value           `json:"homeNotional"`
	ForeignNotional fixedpoint.Value           `json:"foreignNotional"`
	Timestamp       types.MillisecondTimestamp `json:"timestamp"`
}

// todo
func (c *CsvTick) toGlobalTrade() (*types.Trade, error) {
	return &types.Trade{
		ID: c.TradeID,
		// OrderID:    // not implemented
		Exchange:      c.Exchange,
		Price:         c.Price,
		Quantity:      c.Size,
		QuoteQuantity: c.Price.Mul(c.Size), // todo this does not seem right use of propert.. looses info on foreign notional
		Symbol:        c.Symbol,
		Side:          c.Side,
		IsBuyer:       c.Side == types.SideTypeBuy,
		// IsMaker:       isMaker, // todo property isBuyer and isMaker seem to get confused and duplicated
		Time: types.Time(c.Timestamp),
		// Fee:           trade.ExecFee, // todo how to get this info?
		// FeeCurrency:   trade.FeeTokenId,
		IsMargin:   false,
		IsFutures:  false, // todo make future dataset source type as config
		IsIsolated: false,
	}, nil
}
