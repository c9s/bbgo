package csvsource

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type CsvTick struct {
	Exchange        types.ExchangeName `json:"exchange"`
	Market          types.MarketType   `json:"market"`
	TradeID         uint64             `json:"tradeID"`
	Symbol          string             `json:"symbol"`
	TickDirection   string             `json:"tickDirection"`
	Side            types.SideType     `json:"side"`
	IsBuyerMaker    bool
	Size            fixedpoint.Value           `json:"size"`
	Price           fixedpoint.Value           `json:"price"`
	HomeNotional    fixedpoint.Value           `json:"homeNotional"`
	ForeignNotional fixedpoint.Value           `json:"foreignNotional"`
	Timestamp       types.MillisecondTimestamp `json:"timestamp"`
}

func (c *CsvTick) ToGlobalTrade() (*types.Trade, error) {
	var isFutures bool
	if c.Market == types.MarketTypeFutures {
		isFutures = true
	}
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
		IsMaker:       c.IsBuyerMaker,
		Time:          types.Time(c.Timestamp),
		// Fee:           trade.ExecFee, // info is overwritten by stream?
		// FeeCurrency:   trade.FeeTokenId,
		IsFutures:  isFutures,
		IsMargin:   false,
		IsIsolated: false,
	}, nil
}
