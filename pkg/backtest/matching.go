package backtest

import (
	"sync/atomic"
	"time"

	"github.com/pkg/errors"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

var orderID uint64 = 1

func incOrderID() uint64 {
	return atomic.AddUint64(&orderID, 1)
}

// SimplePriceMatching implements a simple kline data driven matching engine for backtest
type SimplePriceMatching struct {
	Symbol string

	bidOrders []types.Order
	askOrders []types.Order

	LastPrice   fixedpoint.Value
	CurrentTime time.Time
}

func (m *SimplePriceMatching) CancelOrder(o types.Order) error {
	found := false

	switch o.Side {

	case types.SideTypeBuy:
		var orders []types.Order
		for _, order := range m.bidOrders {
			if o.OrderID == order.OrderID {
				found = true
				continue
			}
			orders = append(orders, order)
		}
		m.bidOrders = orders

	case types.SideTypeSell:
		var orders []types.Order
		for _, order := range m.bidOrders {
			if o.OrderID == order.OrderID {
				found = true
				continue
			}
			orders = append(orders, order)
		}
		m.bidOrders = orders

	}

	if !found {
		return errors.Errorf("cancel order failed, order not found")
	}
	return nil
}

func (m *SimplePriceMatching) PlaceOrder(o types.SubmitOrder) (closedOrders *types.Order, trades *types.Trade, err error) {
	// start from one
	orderID := incOrderID()

	if o.Type == types.OrderTypeMarket {
		order := newOrder(o, orderID, m.CurrentTime)
		order.Status = types.OrderStatusFilled
		order.ExecutedQuantity = order.Quantity
		order.Price = m.LastPrice.Float64()

		trade := m.newTradeFromOrder(order, false)
		return &order, &trade, nil
	}

	order := newOrder(o, orderID, m.CurrentTime)
	switch o.Side {

	case types.SideTypeBuy:
		m.bidOrders = append(m.bidOrders, order)

	case types.SideTypeSell:
		m.askOrders = append(m.askOrders, order)

	}

	return &order, nil, nil
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

func (m *SimplePriceMatching) BindStream(stream types.Stream) {
	stream.OnKLineClosed(func(kline types.KLine) {
		if kline.Interval != types.Interval1m {
			return
		}
		if kline.Symbol != m.Symbol {
			return
		}

		m.CurrentTime = kline.EndTime

		switch kline.GetTrend() {
		case types.TrendDown:
			if kline.High > kline.Open {
				m.BuyToPrice(fixedpoint.NewFromFloat(kline.High))
			}

			if kline.Low > kline.Close {
				m.SellToPrice(fixedpoint.NewFromFloat(kline.Low))
			}
			m.SellToPrice(fixedpoint.NewFromFloat(kline.Close))

		case types.TrendUp:
			if kline.Low < kline.Open {
				m.SellToPrice(fixedpoint.NewFromFloat(kline.Low))
			}

			if kline.High > kline.Close {
				m.BuyToPrice(fixedpoint.NewFromFloat(kline.High))
			}
			m.BuyToPrice(fixedpoint.NewFromFloat(kline.Close))
		}
	})
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
