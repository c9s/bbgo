package backtest

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"

	"github.com/c9s/bbgo/pkg/bbgo"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

// DefaultFeeRate set the fee rate for most cases
// BINANCE uses 0.1% for both maker and taker
// MAX uses 0.050% for maker and 0.15% for taker
const DefaultFeeRate = 0.15 * 0.001

var orderID uint64 = 1
var tradeID uint64 = 1

func incOrderID() uint64 {
	return atomic.AddUint64(&orderID, 1)
}

func incTradeID() uint64 {
	return atomic.AddUint64(&tradeID, 1)
}

// SimplePriceMatching implements a simple kline data driven matching engine for backtest
type SimplePriceMatching struct {
	Symbol string

	mu        sync.Mutex
	bidOrders []types.Order
	askOrders []types.Order

	LastPrice   fixedpoint.Value
	CurrentTime time.Time

	Account bbgo.BacktestAccount
}

func (m *SimplePriceMatching) CancelOrder(o types.Order) (types.Order, error) {
	found := false

	switch o.Side {

	case types.SideTypeBuy:
		m.mu.Lock()
		var orders []types.Order
		for _, order := range m.bidOrders {
			if o.OrderID == order.OrderID {
				found = true
				continue
			}
			orders = append(orders, order)
		}
		m.bidOrders = orders
		m.mu.Unlock()

	case types.SideTypeSell:
		m.mu.Lock()
		var orders []types.Order
		for _, order := range m.bidOrders {
			if o.OrderID == order.OrderID {
				found = true
				continue
			}
			orders = append(orders, order)
		}
		m.bidOrders = orders
		m.mu.Unlock()

	}

	if !found {
		return o, errors.Errorf("cancel order failed, order not found")
	}

	o.Status = types.OrderStatusCanceled
	return o, nil
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
		m.mu.Lock()
		m.bidOrders = append(m.bidOrders, order)
		m.mu.Unlock()

	case types.SideTypeSell:
		m.mu.Lock()
		m.askOrders = append(m.askOrders, order)
		m.mu.Unlock()
	}

	return &order, nil, nil
}

func (m *SimplePriceMatching) newTradeFromOrder(order types.Order, isMaker bool) types.Trade {
	// BINANCE uses 0.1% for both maker and taker
	// MAX uses 0.050% for maker and 0.15% for taker
	var commission = DefaultFeeRate
	if isMaker && m.Account.MakerCommission > 0 {
		commission = 0.0001 * float64(m.Account.MakerCommission) // binance uses 10~15
	} else if m.Account.TakerCommission > 0 {
		commission = 0.0001 * float64(m.Account.TakerCommission) // binance uses 10~15
	}

	var id = incTradeID()
	return types.Trade{
		ID:            int64(id),
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
		Fee:           order.Quantity * order.Price * commission,
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

func emitTxn(stream *Stream, trades []types.Trade, orders []types.Order) {
	for _, t := range trades {
		stream.EmitTradeUpdate(t)
	}
	for _, o := range orders {
		stream.EmitOrderUpdate(o)
	}
}
func (m *SimplePriceMatching) processKLine(stream *Stream, kline types.KLine) {
	m.CurrentTime = kline.EndTime

	switch kline.GetTrend() {
	case types.TrendDown:
		if kline.High > kline.Open {
			orders, trades := m.BuyToPrice(fixedpoint.NewFromFloat(kline.High))
			emitTxn(stream, trades, orders)
		}

		if kline.Low > kline.Close {
			orders, trades := m.SellToPrice(fixedpoint.NewFromFloat(kline.Low))
			emitTxn(stream, trades, orders)
		}
		orders, trades := m.SellToPrice(fixedpoint.NewFromFloat(kline.Close))
		emitTxn(stream, trades, orders)

	case types.TrendUp:
		if kline.Low < kline.Open {
			orders, trades := m.SellToPrice(fixedpoint.NewFromFloat(kline.Low))
			emitTxn(stream, trades, orders)
		}

		if kline.High > kline.Close {
			orders, trades := m.BuyToPrice(fixedpoint.NewFromFloat(kline.High))
			emitTxn(stream, trades, orders)
		}
		orders, trades := m.BuyToPrice(fixedpoint.NewFromFloat(kline.Close))
		emitTxn(stream, trades, orders)
	}
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
		OrderID:          orderID,
		SubmitOrder:      o,
		Exchange:         "backtest",
		Status:           types.OrderStatusNew,
		ExecutedQuantity: 0,
		IsWorking:        false,
		CreationTime:     creationTime,
		UpdateTime:       creationTime,
	}
}
