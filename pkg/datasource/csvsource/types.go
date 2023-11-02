package csvsource

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type CsvTick struct {
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
		// ID:            tradeIdNum,
		// OrderID:       orderIdNum,
		// Exchange:      types.ExchangeBybit,
		// Price:         trade.OrderPrice,
		// Quantity:      trade.OrderQty,
		// QuoteQuantity: trade.OrderPrice.Mul(trade.OrderQty),
		// Symbol:        trade.Symbol,
		// Side:          side,
		// IsBuyer:       side == types.SideTypeBuy,
		// IsMaker:       isMaker,
		// Time:          types.Time(trade.ExecutionTime),
		// Fee:           trade.ExecFee,
		// FeeCurrency:   trade.FeeTokenId,
		// IsMargin:      false,
		// IsFutures:     false,
		// IsIsolated:    false,
	}, nil
}
