package bitfinex

import (
	"strconv"

	"github.com/c9s/bbgo/pkg/exchange/bitfinex/bfxapi"
	"github.com/c9s/bbgo/pkg/types"
)

// convertOrder converts bfxapi.Order to types.Order
func convertOrder(o bfxapi.Order) (*types.Order, error) {
	// map bfxapi.Order to types.Order using struct literal
	order := &types.Order{
		SubmitOrder: types.SubmitOrder{
			Symbol:   o.Symbol,
			Price:    o.Price,
			Quantity: o.AmountOrig,
			Type:     convertOrderType(o.OrderType),

			AveragePrice: o.PriceAvg,
		},
		OrderID:          uint64(o.OrderID),
		ExecutedQuantity: o.AmountOrig.Sub(o.Amount),
		Status:           convertOrderStatus(o.Status),
		CreationTime:     types.Time(o.CreatedAt),
		UpdateTime:       types.Time(o.UpdatedAt),
		UUID:             "", // Bitfinex does not provide UUID field
		Exchange:         types.ExchangeBitfinex,
	}

	// map ClientOrderID if present
	if o.ClientOrderID != nil {
		order.ClientOrderID = strconv.FormatInt(*o.ClientOrderID, 10)
	}

	// set IsWorking based on status
	order.IsWorking = order.Status == types.OrderStatusNew || order.Status == types.OrderStatusPartiallyFilled

	return order, nil
}

// convertOrderStatus maps bfxapi.OrderStatus to types.OrderStatus
func convertOrderStatus(status bfxapi.OrderStatus) types.OrderStatus {
	// normalize and map Bitfinex order status to bbgo order status
	switch status {
	case bfxapi.OrderStatusActive:
		return types.OrderStatusNew
	case bfxapi.OrderStatusExecuted:
		return types.OrderStatusFilled
	case bfxapi.OrderStatusPartiallyFilled:
		return types.OrderStatusPartiallyFilled
	case bfxapi.OrderStatusCanceled, bfxapi.OrderStatusPartiallyCanceled:
		return types.OrderStatusCanceled
	case bfxapi.OrderStatusRejected:
		return types.OrderStatusRejected
	case bfxapi.OrderStatusExpired:
		return types.OrderStatusExpired
	default:
		// fallback: treat unknown status as rejected
		return types.OrderStatusRejected
	}
}

// convertTrade converts bfxapi.OrderTradeDetail to types.Trade
func convertTrade(trade bfxapi.OrderTradeDetail) (*types.Trade, error) {
	// map bfxapi.OrderTradeDetail to types.Trade using struct literal
	return &types.Trade{
		ID:          uint64(trade.TradeID),
		OrderID:     uint64(trade.OrderID),
		Exchange:    types.ExchangeBitfinex,
		Price:       trade.ExecPrice,
		Quantity:    trade.ExecAmount,
		Symbol:      trade.Symbol,
		IsMaker:     trade.Maker == 1,
		Time:        types.Time(trade.Time),
		Fee:         trade.Fee,
		FeeCurrency: trade.FeeCurrency,
		// ClientOrderID is not present in types.Trade, so skip
	}, nil
}

// convertTicker converts bfxapi.Ticker to types.Ticker.
// It maps Bitfinex ticker fields to the standard types.Ticker fields.
func convertTicker(t bfxapi.Ticker) (*types.Ticker, error) {
	return &types.Ticker{
		Volume: t.Volume,
		Last:   t.LastPrice,
		High:   t.High,
		Low:    t.Low,
		Buy:    t.Bid,
		Sell:   t.Ask,
	}, nil
}

// convertOrderType maps bfxapi.OrderType to types.OrderType.
// It normalizes Bitfinex order type string to bbgo's types.OrderType.
func convertOrderType(t bfxapi.OrderType) types.OrderType {
	switch t {
	case bfxapi.OrderTypeLimit, bfxapi.OrderTypeExchangeLimit:
		return types.OrderTypeLimit
	case bfxapi.OrderTypeMarket, bfxapi.OrderTypeExchangeMarket:
		return types.OrderTypeMarket
	case bfxapi.OrderTypeStopLimit, bfxapi.OrderTypeExchangeStopLimit:
		return types.OrderTypeStopLimit
	case bfxapi.OrderTypeStop, bfxapi.OrderTypeExchangeStop:
		return types.OrderTypeStopMarket
	case bfxapi.OrderTypeTrailingStop, bfxapi.OrderTypeExchangeTrailingStop:
		return types.OrderTypeStopMarket // fallback to stop market
	case bfxapi.OrderTypeFOK, bfxapi.OrderTypeExchangeFOK:
		return types.OrderTypeLimit // fallback to limit
	case bfxapi.OrderTypeIOC, bfxapi.OrderTypeExchangeIOC:
		return types.OrderTypeLimit // fallback to limit
	default:
		return types.OrderTypeLimit // fallback to limit
	}
}
