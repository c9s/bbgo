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
	"github.com/c9s/bbgo/pkg/util"
)

var orderID uint64 = 1
var tradeID uint64 = 1

func incOrderID() uint64 {
	return atomic.AddUint64(&orderID, 1)
}

func incTradeID() uint64 {
	return atomic.AddUint64(&tradeID, 1)
}

var klineMatchingLogger *logrus.Entry = nil

// FeeToken is used to simulate the exchange platform fee token
// This is to ease the back-testing environment for closing positions.
const FeeToken = "FEE"

var useFeeToken = true

func init() {
	logger := logrus.New()
	if v, ok := util.GetEnvVarBool("DEBUG_MATCHING"); ok && v {
		logger.SetLevel(logrus.DebugLevel)
	} else {
		logger.SetLevel(logrus.ErrorLevel)
	}
	klineMatchingLogger = logger.WithField("backtest", "klineEngine")

	if v, ok := util.GetEnvVarBool("BACKTEST_USE_FEE_TOKEN"); ok {
		useFeeToken = v
	}
}

// SimplePriceMatching implements a simple kline data driven matching engine for backtest
//go:generate callbackgen -type SimplePriceMatching
type SimplePriceMatching struct {
	Symbol string
	Market types.Market

	mu           sync.Mutex
	bidOrders    []types.Order
	askOrders    []types.Order
	closedOrders []types.Order

	LastPrice   fixedpoint.Value
	LastKLine   types.KLine
	CurrentTime time.Time

	Account *types.Account

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
		return o, fmt.Errorf("cancel order failed, order %d not found: %+v", o.OrderID, o)
	}

	switch o.Side {
	case types.SideTypeBuy:
		if err := m.Account.UnlockBalance(m.Market.QuoteCurrency, o.Price.Mul(o.Quantity)); err != nil {
			return o, err
		}

	case types.SideTypeSell:
		if err := m.Account.UnlockBalance(m.Market.BaseCurrency, o.Quantity); err != nil {
			return o, err
		}
	}

	o.Status = types.OrderStatusCanceled
	m.EmitOrderUpdate(o)
	m.EmitBalanceUpdate(m.Account.Balances())
	return o, nil
}

func (m *SimplePriceMatching) PlaceOrder(o types.SubmitOrder) (closedOrders *types.Order, trades *types.Trade, err error) {
	// price for checking account balance, default price
	price := o.Price

	switch o.Type {
	case types.OrderTypeMarket:
		if m.LastPrice.IsZero() {
			panic("unexpected: last price can not be zero")
		}

		price = m.LastPrice
	case types.OrderTypeLimit, types.OrderTypeLimitMaker:
		price = o.Price
	}

	if o.Quantity.Compare(m.Market.MinQuantity) < 0 {
		return nil, nil, fmt.Errorf("order quantity %s is less than minQuantity %s, order: %+v", o.Quantity.String(), m.Market.MinQuantity.String(), o)
	}

	quoteQuantity := o.Quantity.Mul(price)
	if quoteQuantity.Compare(m.Market.MinNotional) < 0 {
		return nil, nil, fmt.Errorf("order amount %s is less than minNotional %s, order: %+v", quoteQuantity.String(), m.Market.MinNotional.String(), o)
	}

	switch o.Side {
	case types.SideTypeBuy:
		if err := m.Account.LockBalance(m.Market.QuoteCurrency, quoteQuantity); err != nil {
			return nil, nil, err
		}

	case types.SideTypeSell:
		if err := m.Account.LockBalance(m.Market.BaseCurrency, o.Quantity); err != nil {
			return nil, nil, err
		}
	}

	m.EmitBalanceUpdate(m.Account.Balances())

	// start from one
	orderID := incOrderID()
	order := m.newOrder(o, orderID)

	if o.Type == types.OrderTypeMarket {
		// emit the order update for Status:New
		m.EmitOrderUpdate(order)

		// copy the order object to avoid side effect (for different callbacks)
		var order2 = order

		// emit trade before we publish order
		trade := m.newTradeFromOrder(&order2, false)
		m.executeTrade(trade)

		// update the order status
		order2.Status = types.OrderStatusFilled
		order2.ExecutedQuantity = order2.Quantity
		order2.Price = price
		order2.IsWorking = false

		// let the exchange emit the "FILLED" order update (we need the closed order)
		// m.EmitOrderUpdate(order2)
		return &order2, &trade, nil
	}

	// For limit maker orders (open status)
	// TODO: handle limit taker order
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

	m.EmitOrderUpdate(order) // emit order New status
	return &order, nil, nil
}

func (m *SimplePriceMatching) executeTrade(trade types.Trade) {
	var err error
	// execute trade, update account balances
	if trade.IsBuyer {
		err = m.Account.UseLockedBalance(m.Market.QuoteCurrency, trade.QuoteQuantity)

		// here the fee currency is the base currency
		q := trade.Quantity
		if trade.FeeCurrency == m.Market.BaseCurrency {
			q = q.Sub(trade.Fee)
		}

		m.Account.AddBalance(m.Market.BaseCurrency, q)
	} else {
		err = m.Account.UseLockedBalance(m.Market.BaseCurrency, trade.Quantity)

		// here the fee currency is the quote currency
		qq := trade.QuoteQuantity
		if trade.FeeCurrency == m.Market.QuoteCurrency {
			qq = qq.Sub(trade.Fee)
		}
		m.Account.AddBalance(m.Market.QuoteCurrency, qq)
	}

	if err != nil {
		panic(errors.Wrapf(err, "executeTrade exception, wanted to use more than the locked balance"))
	}

	m.EmitTradeUpdate(trade)
	m.EmitBalanceUpdate(m.Account.Balances())
}

func (m *SimplePriceMatching) newTradeFromOrder(order *types.Order, isMaker bool) types.Trade {
	// BINANCE uses 0.1% for both maker and taker
	// MAX uses 0.050% for maker and 0.15% for taker
	var feeRate fixedpoint.Value
	if isMaker {
		feeRate = m.Account.MakerFeeRate
	} else {
		feeRate = m.Account.TakerFeeRate
	}

	price := order.Price
	switch order.Type {
	case types.OrderTypeMarket, types.OrderTypeStopMarket:
		if m.LastPrice.IsZero() {
			panic("unexpected: last price can not be zero")
		}

		price = m.LastPrice
	}

	var quoteQuantity = order.Quantity.Mul(price)
	var fee fixedpoint.Value
	var feeCurrency string

	if useFeeToken {
		feeCurrency = FeeToken
		fee = quoteQuantity.Mul(feeRate)
	} else {
		switch order.Side {

		case types.SideTypeBuy:
			fee = order.Quantity.Mul(feeRate)
			feeCurrency = m.Market.BaseCurrency

		case types.SideTypeSell:
			fee = quoteQuantity.Mul(feeRate)
			feeCurrency = m.Market.QuoteCurrency

		}
	}

	// update order time
	order.UpdateTime = types.Time(m.CurrentTime)

	var id = incTradeID()
	return types.Trade{
		ID:            id,
		OrderID:       order.OrderID,
		Exchange:      "backtest",
		Price:         price,
		Quantity:      order.Quantity,
		QuoteQuantity: quoteQuantity,
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
	klineMatchingLogger.Debugf("kline buy to price %s", price.String())

	var askOrders []types.Order

	for _, o := range m.askOrders {
		switch o.Type {

		case types.OrderTypeStopMarket:
			// should we trigger the order
			if price.Compare(o.StopPrice) <= 0 {
				// not triggering it, put it back
				askOrders = append(askOrders, o)
				break
			}

			o.Type = types.OrderTypeMarket
			o.ExecutedQuantity = o.Quantity
			o.Price = price
			o.Status = types.OrderStatusFilled
			closedOrders = append(closedOrders, o)

		case types.OrderTypeStopLimit:
			// should we trigger the order?
			if price.Compare(o.StopPrice) <= 0 {
				askOrders = append(askOrders, o)
				break
			}

			o.Type = types.OrderTypeLimit

			// is it a taker order?
			if price.Compare(o.Price) >= 0 {
				if o.Price.Compare(m.LastKLine.Low) < 0 {
					o.Price = m.LastKLine.Low
				}
				o.ExecutedQuantity = o.Quantity
				o.Status = types.OrderStatusFilled
				closedOrders = append(closedOrders, o)
			} else {
				// maker order
				askOrders = append(askOrders, o)
			}

		case types.OrderTypeLimit, types.OrderTypeLimitMaker:
			if price.Compare(o.Price) >= 0 {
				if o.Price.Compare(m.LastKLine.Low) < 0 {
					o.Price = m.LastKLine.Low
				}
				o.ExecutedQuantity = o.Quantity
				o.Status = types.OrderStatusFilled
				closedOrders = append(closedOrders, o)
			} else {
				askOrders = append(askOrders, o)
			}

		default:
			askOrders = append(askOrders, o)
		}

	}

	m.askOrders = askOrders
	m.LastPrice = price

	for i := range closedOrders {
		o := closedOrders[i]
		trade := m.newTradeFromOrder(&o, true)
		m.executeTrade(trade)
		closedOrders[i] = o

		trades = append(trades, trade)

		m.EmitOrderUpdate(o)
	}
	m.closedOrders = append(m.closedOrders, closedOrders...)

	return closedOrders, trades
}

func (m *SimplePriceMatching) SellToPrice(price fixedpoint.Value) (closedOrders []types.Order, trades []types.Trade) {
	klineMatchingLogger.Debugf("kline sell to price %s", price.String())

	var sellPrice = price
	var bidOrders []types.Order
	for _, o := range m.bidOrders {
		switch o.Type {

		case types.OrderTypeStopMarket:
			// should we trigger the order
			if sellPrice.Compare(o.StopPrice) <= 0 {
				o.ExecutedQuantity = o.Quantity
				o.Price = sellPrice
				o.Status = types.OrderStatusFilled
				closedOrders = append(closedOrders, o)
			} else {
				bidOrders = append(bidOrders, o)
			}

		case types.OrderTypeStopLimit:
			// should we trigger the order
			if sellPrice.Compare(o.StopPrice) <= 0 {
				o.Type = types.OrderTypeLimit

				if sellPrice.Compare(o.Price) <= 0 {
					if o.Price.Compare(m.LastKLine.High) > 0 {
						o.Price = m.LastKLine.High
					}
					o.ExecutedQuantity = o.Quantity
					o.Status = types.OrderStatusFilled
					closedOrders = append(closedOrders, o)
				} else {
					bidOrders = append(bidOrders, o)
				}
			} else {
				bidOrders = append(bidOrders, o)
			}

		case types.OrderTypeLimit, types.OrderTypeLimitMaker:
			if sellPrice.Compare(o.Price) <= 0 {
				o.ExecutedQuantity = o.Quantity
				o.Status = types.OrderStatusFilled
				closedOrders = append(closedOrders, o)
			} else {
				bidOrders = append(bidOrders, o)
			}

		default:
			bidOrders = append(bidOrders, o)
		}
	}

	m.bidOrders = bidOrders
	m.LastPrice = price

	for i := range closedOrders {
		o := closedOrders[i]
		trade := m.newTradeFromOrder(&o, true)
		m.executeTrade(trade)
		closedOrders[i] = o

		trades = append(trades, trade)

		m.EmitOrderUpdate(o)
	}
	m.closedOrders = append(m.closedOrders, closedOrders...)

	return closedOrders, trades
}

func (m *SimplePriceMatching) processKLine(kline types.KLine) {
	m.CurrentTime = kline.EndTime.Time()
	m.LastKLine = kline
	if m.LastPrice.IsZero() {
		m.LastPrice = kline.Open
	} else {
		if m.LastPrice.Compare(kline.Open) > 0 {
			m.SellToPrice(kline.Open)
		} else {
			m.BuyToPrice(kline.Open)
		}
	}

	switch kline.Direction() {
	case types.DirectionDown:
		if kline.High.Compare(kline.Open) >= 0 {
			m.BuyToPrice(kline.High)
		}

		// if low is lower than close, sell to low first, and then buy up to close
		if kline.Low.Compare(kline.Close) < 0 {
			m.SellToPrice(kline.Low)
			m.BuyToPrice(kline.Close)
		} else {
			m.SellToPrice(kline.Close)
		}

	case types.DirectionUp:
		if kline.Low.Compare(kline.Open) <= 0 {
			m.SellToPrice(kline.Low)
		}

		if kline.High.Compare(kline.Close) > 0 {
			m.BuyToPrice(kline.High)
			m.SellToPrice(kline.Close)
		} else {
			m.BuyToPrice(kline.Close)
		}
	default: // no trade up or down
		if m.LastPrice.IsZero() {
			m.BuyToPrice(kline.Close)
		}

	}
}

func (m *SimplePriceMatching) newOrder(o types.SubmitOrder, orderID uint64) types.Order {
	return types.Order{
		OrderID:          orderID,
		SubmitOrder:      o,
		Exchange:         types.ExchangeBacktest,
		Status:           types.OrderStatusNew,
		ExecutedQuantity: fixedpoint.Zero,
		IsWorking:        true,
		CreationTime:     types.Time(m.CurrentTime),
		UpdateTime:       types.Time(m.CurrentTime),
	}
}
