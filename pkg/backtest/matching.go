package backtest

import (
	"sort"
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type PriceOrder struct {
	Price fixedpoint.Value
	Order types.Order
}

type PriceOrderSlice []PriceOrder

func (slice PriceOrderSlice) Len() int           { return len(slice) }
func (slice PriceOrderSlice) Less(i, j int) bool { return slice[i].Price < slice[j].Price }
func (slice PriceOrderSlice) Swap(i, j int)      { slice[i], slice[j] = slice[j], slice[i] }

func (slice PriceOrderSlice) InsertAt(idx int, po PriceOrder) PriceOrderSlice {
	rear := append([]PriceOrder{}, slice[idx:]...)
	newSlice := append(slice[:idx], po)
	return append(newSlice, rear...)
}

func (slice PriceOrderSlice) Remove(price fixedpoint.Value, descending bool) PriceOrderSlice {
	matched, idx := slice.Find(price, descending)
	if matched.Price != price {
		return slice
	}

	return append(slice[:idx], slice[idx+1:]...)
}

func (slice PriceOrderSlice) First() (PriceOrder, bool) {
	if len(slice) > 0 {
		return slice[0], true
	}
	return PriceOrder{}, false
}

// FindPriceVolumePair finds the pair by the given price, this function is a read-only
// operation, so we use the value receiver to avoid copy value from the pointer
// If the price is not found, it will return the index where the price can be inserted at.
// true for descending (bid orders), false for ascending (ask orders)
func (slice PriceOrderSlice) Find(price fixedpoint.Value, descending bool) (pv PriceOrder, idx int) {
	idx = sort.Search(len(slice), func(i int) bool {
		if descending {
			return slice[i].Price <= price
		}
		return slice[i].Price >= price
	})

	if idx >= len(slice) || slice[idx].Price != price {
		return pv, idx
	}

	pv = slice[idx]

	return pv, idx
}

func (slice PriceOrderSlice) Upsert(po PriceOrder, descending bool) PriceOrderSlice {
	if len(slice) == 0 {
		return append(slice, po)
	}

	price := po.Price
	_, idx := slice.Find(price, descending)
	if idx >= len(slice) || slice[idx].Price != price {
		return slice.InsertAt(idx, po)
	}

	slice[idx].Order = po.Order
	return slice
}

type SimplePriceMatching struct {
	bidOrders []types.Order
	askOrders []types.Order

	LastPrice   fixedpoint.Value
	CurrentTime time.Time
	OrderID     uint64
}

func (m *SimplePriceMatching) PlaceOrder(o types.SubmitOrder) (closedOrders []types.Order, trades []types.Trade, err error) {
	// start from one
	m.OrderID++

	if o.Type == types.OrderTypeMarket {
		order := newOrder(o, m.OrderID, m.CurrentTime)
		order.Status = types.OrderStatusFilled
		order.ExecutedQuantity = order.Quantity
		order.Price = m.LastPrice.Float64()
		closedOrders = append(closedOrders, order)

		trade := m.newTradeFromOrder(order, false)
		trades = append(trades, trade)
		return
	}

	switch o.Side {

	case types.SideTypeBuy:
		m.bidOrders = append(m.bidOrders, newOrder(o, m.OrderID, m.CurrentTime))

	case types.SideTypeSell:
		m.askOrders = append(m.askOrders, newOrder(o, m.OrderID, m.CurrentTime))

	}

	return
}

func (m *SimplePriceMatching) newTradeFromOrder(order types.Order, isMaker bool) types.Trade {
	return types.Trade{
		ID:            0,
		OrderID:       order.OrderID,
		Exchange:      "backtest",
		Price:         order.Price,
		Quantity:      order.Quantity,
		QuoteQuantity: order.Quantity * order.Price,
		Symbol:        order.Symbol,
		Side:          order.Side,
		IsBuyer:       order.Side == types.SideTypeBuy,
		IsMaker:       isMaker,
		Time:          m.CurrentTime,
		Fee:           order.Quantity * order.Price * 0.0015,
		FeeCurrency:   "USDT",
	}
}

func (m *SimplePriceMatching) BuyToPrice(price fixedpoint.Value) (closedOrders []types.Order, trades []types.Trade) {
	var priceF = price.Float64()
	var askOrders []types.Order
	for _, o := range m.askOrders {
		switch o.Type {

		case types.OrderTypeStopMarket:
			// should we trigger the order
			if priceF >= o.StopPrice {
				o.ExecutedQuantity = o.Quantity
				o.Price = priceF
				o.Status = types.OrderStatusFilled
				closedOrders = append(closedOrders, o)

				trade := m.newTradeFromOrder(o, false)
				trades = append(trades, trade)
			} else {
				askOrders = append(askOrders, o)
			}

		case types.OrderTypeStopLimit:
			// should we trigger the order
			if priceF >= o.StopPrice {
				o.Type = types.OrderTypeLimit

				if priceF >= o.Price {
					o.ExecutedQuantity = o.Quantity
					o.Status = types.OrderStatusFilled
					closedOrders = append(closedOrders, o)

					trade := m.newTradeFromOrder(o, false)
					trades = append(trades, trade)
				} else {
					askOrders = append(askOrders, o)
				}
			} else {
				askOrders = append(askOrders, o)
			}

		case types.OrderTypeLimit:
			if priceF >= o.Price {
				o.ExecutedQuantity = o.Quantity
				o.Status = types.OrderStatusFilled
				closedOrders = append(closedOrders, o)

				trade := m.newTradeFromOrder(o, true)
				trades = append(trades, trade)
			} else {
				askOrders = append(askOrders, o)
			}

		default:
			askOrders = append(askOrders, o)
		}

	}

	m.askOrders = askOrders
	m.LastPrice = price

	return closedOrders, trades
}

func (m *SimplePriceMatching) SellToPrice(price fixedpoint.Value) (closedOrders []types.Order, trades []types.Trade) {
	var sellPrice = price.Float64()
	var bidOrders []types.Order
	for _, o := range m.bidOrders {
		switch o.Type {

		case types.OrderTypeStopMarket:
			// should we trigger the order
			if sellPrice <= o.StopPrice {
				o.ExecutedQuantity = o.Quantity
				o.Price = sellPrice
				o.Status = types.OrderStatusFilled
				closedOrders = append(closedOrders, o)

				trade := m.newTradeFromOrder(o, false)
				trades = append(trades, trade)
			} else {
				bidOrders = append(bidOrders, o)
			}

		case types.OrderTypeStopLimit:
			// should we trigger the order
			if sellPrice <= o.StopPrice {
				o.Type = types.OrderTypeLimit

				if sellPrice <= o.Price {
					o.ExecutedQuantity = o.Quantity
					o.Status = types.OrderStatusFilled
					closedOrders = append(closedOrders, o)

					trade := m.newTradeFromOrder(o, false)
					trades = append(trades, trade)
				} else {
					bidOrders = append(bidOrders, o)
				}
			} else {
				bidOrders = append(bidOrders, o)
			}

		case types.OrderTypeLimit:
			if sellPrice <= o.Price {
				o.ExecutedQuantity = o.Quantity
				o.Status = types.OrderStatusFilled
				closedOrders = append(closedOrders, o)

				trade := m.newTradeFromOrder(o, true)
				trades = append(trades, trade)
			} else {
				bidOrders = append(bidOrders, o)
			}

		default:
			bidOrders = append(bidOrders, o)
		}
	}

	m.bidOrders = bidOrders
	m.LastPrice = price

	return closedOrders, trades
}

type Matching struct {
	Symbol string
	Asks   PriceOrderSlice
	Bids   PriceOrderSlice

	OrderID     uint64
	CurrentTime time.Time
}

func (m *Matching) PlaceOrder(o types.SubmitOrder) {
	var order = types.Order{
		SubmitOrder:      o,
		Exchange:         "backtest",
		OrderID:          m.OrderID,
		Status:           types.OrderStatusNew,
		ExecutedQuantity: 0,
		IsWorking:        false,
		CreationTime:     m.CurrentTime,
		UpdateTime:       m.CurrentTime,
	}
	_ = order
}

func newOrder(o types.SubmitOrder, orderID uint64, creationTime time.Time) types.Order {
	return types.Order{
		SubmitOrder:      o,
		Exchange:         "backtest",
		OrderID:          orderID,
		Status:           types.OrderStatusNew,
		ExecutedQuantity: 0,
		IsWorking:        false,
		CreationTime:     creationTime,
		UpdateTime:       creationTime,
	}
}
