package backtest

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

// DefaultFeeRate set the fee rate for most cases
// BINANCE uses 0.1% for both maker and taker
//  for BNB holders, it's 0.075% for both maker and taker
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
//go:generate callbackgen -type SimplePriceMatching
type SimplePriceMatching struct {
	Symbol string
	Market types.Market

	mu        sync.Mutex
	bidOrders []types.Order
	askOrders []types.Order

	LastPrice   fixedpoint.Value
	LastKLine   types.KLine
	CurrentTime time.Time

	Account *types.Account

	MakerCommission fixedpoint.Value `json:"makerCommission"`
	TakerCommission fixedpoint.Value `json:"takerCommission"`

	tradeUpdateCallbacks   []func(trade types.Trade)
	orderUpdateCallbacks   []func(order types.Order)
	balanceUpdateCallbacks []func(balances types.BalanceMap)
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
		for _, order := range m.askOrders {
			if o.OrderID == order.OrderID {
				found = true
				continue
			}
			orders = append(orders, order)
		}
		m.askOrders = orders
		m.mu.Unlock()

	}

	if !found {
		logrus.Panicf("cancel order failed, order %d not found: %+v", o.OrderID, o)

		return o, fmt.Errorf("cancel order failed, order %d not found: %+v", o.OrderID, o)
	}

	switch o.Side {
	case types.SideTypeBuy:
		if err := m.Account.UnlockBalance(m.Market.QuoteCurrency, fixedpoint.NewFromFloat(o.Price*o.Quantity)); err != nil {
			return o, err
		}

	case types.SideTypeSell:
		if err := m.Account.UnlockBalance(m.Market.BaseCurrency, fixedpoint.NewFromFloat(o.Quantity)); err != nil {
			return o, err
		}
	}

	o.Status = types.OrderStatusCanceled
	m.EmitOrderUpdate(o)
	m.EmitBalanceUpdate(m.Account.Balances())
	return o, nil
}

func (m *SimplePriceMatching) PlaceOrder(o types.SubmitOrder) (closedOrders *types.Order, trades *types.Trade, err error) {

	// price for checking account balance
	price := o.Price
	switch o.Type {
	case types.OrderTypeMarket:
		price = m.LastPrice.Float64()
	case types.OrderTypeLimit:
		price = o.Price
	}

	switch o.Side {
	case types.SideTypeBuy:
		quote := price * o.Quantity
		if err := m.Account.LockBalance(m.Market.QuoteCurrency, fixedpoint.NewFromFloat(quote)); err != nil {
			return nil, nil, err
		}

	case types.SideTypeSell:
		baseQuantity := o.Quantity
		if err := m.Account.LockBalance(m.Market.BaseCurrency, fixedpoint.NewFromFloat(baseQuantity)); err != nil {
			return nil, nil, err
		}
	}

	m.EmitBalanceUpdate(m.Account.Balances())

	// start from one
	orderID := incOrderID()
	order := m.newOrder(o, orderID)

	if o.Type == types.OrderTypeMarket {
		m.EmitOrderUpdate(order)

		// emit trade before we publish order
		trade := m.newTradeFromOrder(order, false)
		m.executeTrade(trade)

		// update the order status
		order.Status = types.OrderStatusFilled
		order.ExecutedQuantity = order.Quantity
		order.Price = price
		m.EmitOrderUpdate(order)
		m.EmitBalanceUpdate(m.Account.Balances())
		return &order, &trade, nil
	}

	// for limit maker orders
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

	m.EmitOrderUpdate(order)

	return &order, nil, nil
}

func (m *SimplePriceMatching) executeTrade(trade types.Trade) {
	var err error
	// execute trade, update account balances
	if trade.IsBuyer {
		err = m.Account.UseLockedBalance(m.Market.QuoteCurrency, fixedpoint.NewFromFloat(trade.Price*trade.Quantity))

		_ = m.Account.AddBalance(m.Market.BaseCurrency, fixedpoint.NewFromFloat(trade.Quantity))
	} else {
		err = m.Account.UseLockedBalance(m.Market.BaseCurrency, fixedpoint.NewFromFloat(trade.Quantity))

		_ = m.Account.AddBalance(m.Market.QuoteCurrency, fixedpoint.NewFromFloat(trade.Quantity*trade.Price))
	}

	if err != nil {
		panic(errors.Wrapf(err, "executeTrade exception, wanted to use more than the locked balance"))
	}

	m.EmitTradeUpdate(trade)
	m.EmitBalanceUpdate(m.Account.Balances())
	return
}

func (m *SimplePriceMatching) newTradeFromOrder(order types.Order, isMaker bool) types.Trade {
	// BINANCE uses 0.1% for both maker and taker
	// MAX uses 0.050% for maker and 0.15% for taker
	var commission = DefaultFeeRate
	if isMaker && m.Account.MakerCommission > 0 {
		commission = fixedpoint.NewFromFloat(0.0001).Mul(m.Account.MakerCommission).Float64() // binance uses 10~15
	} else if m.Account.TakerCommission > 0 {
		commission = fixedpoint.NewFromFloat(0.0001).Mul(m.Account.TakerCommission).Float64() // binance uses 10~15
	}

	var fee float64
	var feeCurrency string

	switch order.Side {

	case types.SideTypeBuy:
		fee = order.Quantity * commission
		feeCurrency = m.Market.BaseCurrency

	case types.SideTypeSell:
		fee = order.Quantity * order.Price * commission
		feeCurrency = m.Market.QuoteCurrency

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
		Time:          types.Time(m.CurrentTime),
		Fee:           fee,
		FeeCurrency:   feeCurrency,
	}
}

func (m *SimplePriceMatching) BuyToPrice(price fixedpoint.Value) (closedOrders []types.Order, trades []types.Trade) {
	var priceF = price.Float64()
	var askOrders []types.Order

	for _, o := range m.askOrders {
		switch o.Type {

		case types.OrderTypeStopMarket:
			// should we trigger the order
			if priceF <= o.StopPrice {
				// not triggering it, put it back
				askOrders = append(askOrders, o)
				break
			}

			o.Type = types.OrderTypeMarket
			o.ExecutedQuantity = o.Quantity
			o.Price = priceF
			o.Status = types.OrderStatusFilled
			closedOrders = append(closedOrders, o)

			trade := m.newTradeFromOrder(o, false)
			m.executeTrade(trade)

			trades = append(trades, trade)

			m.EmitOrderUpdate(o)

		case types.OrderTypeStopLimit:
			// should we trigger the order?
			if priceF <= o.StopPrice {
				askOrders = append(askOrders, o)
				break
			}

			o.Type = types.OrderTypeLimit

			// is it a taker order?
			if priceF >= o.Price {
				o.ExecutedQuantity = o.Quantity
				o.Status = types.OrderStatusFilled
				closedOrders = append(closedOrders, o)

				trade := m.newTradeFromOrder(o, false)
				m.executeTrade(trade)

				trades = append(trades, trade)

				m.EmitOrderUpdate(o)
			} else {
				// maker order
				askOrders = append(askOrders, o)
			}

		case types.OrderTypeLimit:
			if priceF >= o.Price {
				o.ExecutedQuantity = o.Quantity
				o.Status = types.OrderStatusFilled
				closedOrders = append(closedOrders, o)

				trade := m.newTradeFromOrder(o, true)
				m.executeTrade(trade)

				trades = append(trades, trade)

				m.EmitOrderUpdate(o)
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
				m.executeTrade(trade)

				trades = append(trades, trade)

				m.EmitOrderUpdate(o)
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
					m.executeTrade(trade)

					trades = append(trades, trade)
					m.EmitOrderUpdate(o)

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
				m.executeTrade(trade)

				trades = append(trades, trade)

				m.EmitOrderUpdate(o)
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

func (m *SimplePriceMatching) processKLine(kline types.KLine) {
	m.CurrentTime = kline.EndTime
	m.LastKLine = kline

	switch kline.Direction() {
	case types.DirectionDown:
		if kline.High > kline.Open {
			m.BuyToPrice(fixedpoint.NewFromFloat(kline.High))
		}

		if kline.Low > kline.Close {
			m.SellToPrice(fixedpoint.NewFromFloat(kline.Low))
			m.BuyToPrice(fixedpoint.NewFromFloat(kline.Close))
		} else {
			m.SellToPrice(fixedpoint.NewFromFloat(kline.Close))
		}

	case types.DirectionUp:
		if kline.Low < kline.Open {
			m.SellToPrice(fixedpoint.NewFromFloat(kline.Low))
		}

		if kline.High > kline.Close {
			m.BuyToPrice(fixedpoint.NewFromFloat(kline.High))
			m.SellToPrice(fixedpoint.NewFromFloat(kline.Close))
		} else {
			m.BuyToPrice(fixedpoint.NewFromFloat(kline.Close))
		}

	}
}

func (m *SimplePriceMatching) newOrder(o types.SubmitOrder, orderID uint64) types.Order {
	return types.Order{
		OrderID:          orderID,
		SubmitOrder:      o,
		Exchange:         types.ExchangeBacktest,
		Status:           types.OrderStatusNew,
		ExecutedQuantity: 0,
		IsWorking:        true,
		CreationTime:     types.Time(m.CurrentTime),
		UpdateTime:       types.Time(m.CurrentTime),
	}
}
