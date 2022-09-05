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
	closedOrders map[uint64]types.Order

	klineCache  map[types.Interval]types.KLine
	lastPrice   fixedpoint.Value
	lastKLine   types.KLine
	nextKLine   *types.KLine
	currentTime time.Time

	feeModeFunction FeeModeFunction

	account *types.Account

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
		if err := m.account.UnlockBalance(m.Market.QuoteCurrency, o.Price.Mul(o.Quantity)); err != nil {
			return o, err
		}

	case types.SideTypeSell:
		if err := m.account.UnlockBalance(m.Market.BaseCurrency, o.Quantity); err != nil {
			return o, err
		}
	}

	o.Status = types.OrderStatusCanceled
	m.EmitOrderUpdate(o)
	m.EmitBalanceUpdate(m.account.Balances())
	return o, nil
}

// PlaceOrder returns the created order object, executed trade (if any) and error
func (m *SimplePriceMatching) PlaceOrder(o types.SubmitOrder) (*types.Order, *types.Trade, error) {
	if o.Type == types.OrderTypeMarket {
		if m.lastPrice.IsZero() {
			panic("unexpected error: for market order, the last price can not be zero")
		}
	}

	isTaker := o.Type == types.OrderTypeMarket || isLimitTakerOrder(o, m.lastPrice)

	// price for checking account balance, default price
	price := o.Price

	switch o.Type {
	case types.OrderTypeMarket:
		price = m.Market.TruncatePrice(m.lastPrice)

	case types.OrderTypeStopMarket:
		// the actual price might be different.
		o.StopPrice = m.Market.TruncatePrice(o.StopPrice)
		price = o.StopPrice

	case types.OrderTypeLimit, types.OrderTypeStopLimit, types.OrderTypeLimitMaker:
		o.Price = m.Market.TruncatePrice(o.Price)
		price = o.Price
	}

	o.Quantity = m.Market.TruncateQuantity(o.Quantity)

	if o.Quantity.Compare(m.Market.MinQuantity) < 0 {
		return nil, nil, fmt.Errorf("order quantity %s is less than minQuantity %s, order: %+v", o.Quantity.String(), m.Market.MinQuantity.String(), o)
	}

	quoteQuantity := o.Quantity.Mul(price)
	if quoteQuantity.Compare(m.Market.MinNotional) < 0 {
		return nil, nil, fmt.Errorf("order amount %s is less than minNotional %s, order: %+v", quoteQuantity.String(), m.Market.MinNotional.String(), o)
	}

	switch o.Side {
	case types.SideTypeBuy:
		if err := m.account.LockBalance(m.Market.QuoteCurrency, quoteQuantity); err != nil {
			return nil, nil, err
		}

	case types.SideTypeSell:
		if err := m.account.LockBalance(m.Market.BaseCurrency, o.Quantity); err != nil {
			return nil, nil, err
		}
	}

	m.EmitBalanceUpdate(m.account.Balances())

	// start from one
	orderID := incOrderID()
	order := m.newOrder(o, orderID)

	if isTaker {
		var price fixedpoint.Value
		if order.Type == types.OrderTypeMarket {
			order.Price = m.Market.TruncatePrice(m.lastPrice)
			price = order.Price
		} else if order.Type == types.OrderTypeLimit {
			// if limit order's price is with the range of next kline
			// we assume it will be traded as a maker trade, and is traded at its original price
			// TODO: if it is treated as a maker trade, fee should be specially handled
			// otherwise, set NextKLine.Close(i.e., m.LastPrice) to be the taker traded price
			if m.nextKLine != nil && m.nextKLine.High.Compare(order.Price) > 0 && order.Side == types.SideTypeBuy {
				order.AveragePrice = order.Price
			} else if m.nextKLine != nil && m.nextKLine.Low.Compare(order.Price) < 0 && order.Side == types.SideTypeSell {
				order.AveragePrice = order.Price
			} else {
				order.AveragePrice = m.Market.TruncatePrice(m.lastPrice)
			}
			price = order.AveragePrice
		}

		// emit the order update for Status:New
		m.EmitOrderUpdate(order)

		// copy the order object to avoid side effect (for different callbacks)
		var order2 = order

		// emit trade before we publish order
		trade := m.newTradeFromOrder(&order2, false, price)
		m.executeTrade(trade)

		// unlock the rest balances for limit taker
		if order.Type == types.OrderTypeLimit {
			if order.AveragePrice.IsZero() {
				return nil, nil, fmt.Errorf("the average price of the given limit taker order can not be zero")
			}

			switch o.Side {
			case types.SideTypeBuy:
				// limit buy taker, the order price is higher than the current best ask price
				// the executed price is lower than the given price, so we will use less quote currency to buy the base asset.
				amount := order.Price.Sub(order.AveragePrice).Mul(order.Quantity)
				if amount.Sign() > 0 {
					if err := m.account.UnlockBalance(m.Market.QuoteCurrency, amount); err != nil {
						return nil, nil, err
					}
					m.EmitBalanceUpdate(m.account.Balances())
				}

			case types.SideTypeSell:
				// limit sell taker, the order price is lower than the current best bid price
				// the executed price is higher than the given price, so we will get more quote currency back
				amount := order.AveragePrice.Sub(order.Price).Mul(order.Quantity)
				if amount.Sign() > 0 {
					m.account.AddBalance(m.Market.QuoteCurrency, amount)
					m.EmitBalanceUpdate(m.account.Balances())
				}
			}
		}

		// update the order status
		order2.Status = types.OrderStatusFilled
		order2.ExecutedQuantity = order2.Quantity
		order2.IsWorking = false
		m.EmitOrderUpdate(order2)

		// let the exchange emit the "FILLED" order update (we need the closed order)
		// m.EmitOrderUpdate(order2)
		return &order2, &trade, nil
	}

	// For limit maker orders (open status)
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
		err = m.account.UseLockedBalance(m.Market.QuoteCurrency, trade.QuoteQuantity)

		// all-in buy trade, we can only deduct the fee from the quote quantity and re-calculate the base quantity
		switch trade.FeeCurrency {
		case m.Market.QuoteCurrency:
			m.account.AddBalance(m.Market.QuoteCurrency, trade.Fee.Neg())
			m.account.AddBalance(m.Market.BaseCurrency, trade.Quantity)
		case m.Market.BaseCurrency:
			m.account.AddBalance(m.Market.BaseCurrency, trade.Quantity.Sub(trade.Fee))
		default:
			m.account.AddBalance(m.Market.BaseCurrency, trade.Quantity)
		}

	} else { // sell trade
		err = m.account.UseLockedBalance(m.Market.BaseCurrency, trade.Quantity)

		switch trade.FeeCurrency {
		case m.Market.QuoteCurrency:
			m.account.AddBalance(m.Market.QuoteCurrency, trade.QuoteQuantity.Sub(trade.Fee))
		case m.Market.BaseCurrency:
			m.account.AddBalance(m.Market.BaseCurrency, trade.Fee.Neg())
			m.account.AddBalance(m.Market.QuoteCurrency, trade.QuoteQuantity)
		default:
			m.account.AddBalance(m.Market.QuoteCurrency, trade.QuoteQuantity)
		}
	}

	if err != nil {
		panic(errors.Wrapf(err, "executeTrade exception, wanted to use more than the locked balance"))
	}

	m.EmitTradeUpdate(trade)
	m.EmitBalanceUpdate(m.account.Balances())
}

func (m *SimplePriceMatching) getFeeRate(isMaker bool) (feeRate fixedpoint.Value) {
	// BINANCE uses 0.1% for both maker and taker
	// MAX uses 0.050% for maker and 0.15% for taker
	if isMaker {
		feeRate = m.account.MakerFeeRate
	} else {
		feeRate = m.account.TakerFeeRate
	}
	return feeRate
}

func (m *SimplePriceMatching) newTradeFromOrder(order *types.Order, isMaker bool, price fixedpoint.Value) types.Trade {
	// BINANCE uses 0.1% for both maker and taker
	// MAX uses 0.050% for maker and 0.15% for taker
	var feeRate = m.getFeeRate(isMaker)
	var quoteQuantity = order.Quantity.Mul(price)
	var fee fixedpoint.Value
	var feeCurrency string

	if m.feeModeFunction != nil {
		fee, feeCurrency = m.feeModeFunction(order, &m.Market, feeRate)
	} else {
		fee, feeCurrency = feeModeFunctionQuote(order, &m.Market, feeRate)
	}

	// update order time
	order.UpdateTime = types.Time(m.currentTime)

	var id = incTradeID()
	return types.Trade{
		ID:            id,
		OrderID:       order.OrderID,
		Exchange:      types.ExchangeBacktest,
		Price:         price,
		Quantity:      order.Quantity,
		QuoteQuantity: quoteQuantity,
		Symbol:        order.Symbol,
		Side:          order.Side,
		IsBuyer:       order.Side == types.SideTypeBuy,
		IsMaker:       isMaker,
		Time:          types.Time(m.currentTime),
		Fee:           fee,
		FeeCurrency:   feeCurrency,
	}
}

// buyToPrice means price go up and the limit sell should be triggered
func (m *SimplePriceMatching) buyToPrice(price fixedpoint.Value) (closedOrders []types.Order, trades []types.Trade) {
	klineMatchingLogger.Debugf("kline buy to price %s", price.String())

	var bidOrders []types.Order
	for _, o := range m.bidOrders {
		switch o.Type {

		case types.OrderTypeStopMarket:
			// the price is still lower than the stop price, we will put the order back to the list
			if price.Compare(o.StopPrice) < 0 {
				// not triggering it, put it back
				bidOrders = append(bidOrders, o)
				break
			}

			o.Type = types.OrderTypeMarket
			o.ExecutedQuantity = o.Quantity
			o.Price = price
			o.Status = types.OrderStatusFilled
			closedOrders = append(closedOrders, o)

		case types.OrderTypeStopLimit:
			// the price is still lower than the stop price, we will put the order back to the list
			if price.Compare(o.StopPrice) < 0 {
				bidOrders = append(bidOrders, o)
				break
			}

			// convert this order to limit order
			// we use value object here, so it's a copy
			o.Type = types.OrderTypeLimit

			// is it a taker order?
			// higher than the current price, then it's a taker order
			if o.Price.Compare(price) >= 0 {
				// limit buy taker order, move it to the closed order
				// we assume that we have no price slippage here, so the latest price will be the executed price
				o.AveragePrice = price
				o.ExecutedQuantity = o.Quantity
				o.Status = types.OrderStatusFilled
				closedOrders = append(closedOrders, o)
			} else {
				// keep it as a maker order
				bidOrders = append(bidOrders, o)
			}
		default:
			bidOrders = append(bidOrders, o)
		}
	}
	m.bidOrders = bidOrders

	var askOrders []types.Order
	for _, o := range m.askOrders {
		switch o.Type {

		case types.OrderTypeStopMarket:
			// should we trigger the order
			if price.Compare(o.StopPrice) < 0 {
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
			if price.Compare(o.StopPrice) < 0 {
				askOrders = append(askOrders, o)
				break
			}

			o.Type = types.OrderTypeLimit

			// is it a taker order?
			// higher than the current price, then it's a taker order
			if o.Price.Compare(price) <= 0 {
				// limit sell order as taker, move it to the closed order
				// we assume that we have no price slippage here, so the latest price will be the executed price
				// TODO: simulate slippage here
				o.AveragePrice = price
				o.ExecutedQuantity = o.Quantity
				o.Status = types.OrderStatusFilled
				closedOrders = append(closedOrders, o)
			} else {
				// maker order
				askOrders = append(askOrders, o)
			}

		case types.OrderTypeLimit, types.OrderTypeLimitMaker:
			if price.Compare(o.Price) >= 0 {
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
	m.lastPrice = price

	for i := range closedOrders {
		o := closedOrders[i]
		executedPrice := o.Price
		if !o.AveragePrice.IsZero() {
			executedPrice = o.AveragePrice
		}

		trade := m.newTradeFromOrder(&o, !isTakerOrder(o), executedPrice)
		m.executeTrade(trade)
		closedOrders[i] = o

		trades = append(trades, trade)

		m.EmitOrderUpdate(o)

		m.closedOrders[o.OrderID] = o
	}

	return closedOrders, trades
}

// sellToPrice simulates the price trend in down direction.
// When price goes down, buy orders should be executed, and the stop orders should be triggered.
func (m *SimplePriceMatching) sellToPrice(price fixedpoint.Value) (closedOrders []types.Order, trades []types.Trade) {
	klineMatchingLogger.Debugf("kline sell to price %s", price.String())

	// in this section we handle --- the price goes lower, and we trigger the stop sell
	var askOrders []types.Order
	for _, o := range m.askOrders {
		switch o.Type {

		case types.OrderTypeStopMarket:
			// should we trigger the order
			if price.Compare(o.StopPrice) > 0 {
				askOrders = append(askOrders, o)
				break
			}

			o.Type = types.OrderTypeMarket
			o.ExecutedQuantity = o.Quantity
			o.Price = price
			o.Status = types.OrderStatusFilled
			closedOrders = append(closedOrders, o)

		case types.OrderTypeStopLimit:
			// if the price is lower than the stop price
			// we should trigger the stop sell order
			if price.Compare(o.StopPrice) > 0 {
				askOrders = append(askOrders, o)
				break
			}

			o.Type = types.OrderTypeLimit

			// handle TAKER SELL
			// if the order price is lower than the current price
			// it's a taker order
			if o.Price.Compare(price) <= 0 {
				o.AveragePrice = price
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

	var bidOrders []types.Order
	for _, o := range m.bidOrders {
		switch o.Type {

		case types.OrderTypeStopMarket:
			// price goes down and if the stop price is still lower than the current price
			// or the stop price is not touched
			// then we should skip this order
			if price.Compare(o.StopPrice) > 0 {
				bidOrders = append(bidOrders, o)
				break
			}

			o.Type = types.OrderTypeMarket
			o.ExecutedQuantity = o.Quantity
			o.Price = price
			o.Status = types.OrderStatusFilled
			closedOrders = append(closedOrders, o)

		case types.OrderTypeStopLimit:
			// price goes down and if the stop price is still lower than the current price
			// or the stop price is not touched
			// then we should skip this order
			if price.Compare(o.StopPrice) > 0 {
				bidOrders = append(bidOrders, o)
				break
			}

			o.Type = types.OrderTypeLimit

			// handle TAKER order
			if o.Price.Compare(price) >= 0 {
				o.AveragePrice = price
				o.ExecutedQuantity = o.Quantity
				o.Status = types.OrderStatusFilled
				closedOrders = append(closedOrders, o)
			} else {
				bidOrders = append(bidOrders, o)
			}

		case types.OrderTypeLimit, types.OrderTypeLimitMaker:
			if price.Compare(o.Price) <= 0 {
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
	m.lastPrice = price

	for i := range closedOrders {
		o := closedOrders[i]
		executedPrice := o.Price
		if !o.AveragePrice.IsZero() {
			executedPrice = o.AveragePrice
		}

		trade := m.newTradeFromOrder(&o, !isTakerOrder(o), executedPrice)
		m.executeTrade(trade)
		closedOrders[i] = o

		trades = append(trades, trade)

		m.EmitOrderUpdate(o)

		m.closedOrders[o.OrderID] = o
	}

	return closedOrders, trades
}

func (m *SimplePriceMatching) getOrder(orderID uint64) (types.Order, bool) {
	if o, ok := m.closedOrders[orderID]; ok {
		return o, true
	}

	for _, o := range m.bidOrders {
		if o.OrderID == orderID {
			return o, true
		}
	}

	for _, o := range m.askOrders {
		if o.OrderID == orderID {
			return o, true
		}
	}

	return types.Order{}, false
}

func (m *SimplePriceMatching) processKLine(kline types.KLine) {
	m.currentTime = kline.EndTime.Time()

	if m.lastPrice.IsZero() {
		m.lastPrice = kline.Open
	} else {
		if m.lastPrice.Compare(kline.Open) > 0 {
			m.sellToPrice(kline.Open)
		} else {
			m.buyToPrice(kline.Open)
		}
	}

	switch kline.Direction() {
	case types.DirectionDown:
		if kline.High.Compare(kline.Open) >= 0 {
			m.buyToPrice(kline.High)
		}

		// if low is lower than close, sell to low first, and then buy up to close
		if kline.Low.Compare(kline.Close) < 0 {
			m.sellToPrice(kline.Low)
			m.buyToPrice(kline.Close)
		} else {
			m.sellToPrice(kline.Close)
		}

	case types.DirectionUp:
		if kline.Low.Compare(kline.Open) <= 0 {
			m.sellToPrice(kline.Low)
		}

		if kline.High.Compare(kline.Close) > 0 {
			m.buyToPrice(kline.High)
			m.sellToPrice(kline.Close)
		} else {
			m.buyToPrice(kline.Close)
		}
	default: // no trade up or down
		if m.lastPrice.IsZero() {
			m.buyToPrice(kline.Close)
		}
	}

	m.lastKLine = kline
}

func (m *SimplePriceMatching) newOrder(o types.SubmitOrder, orderID uint64) types.Order {
	return types.Order{
		OrderID:          orderID,
		SubmitOrder:      o,
		Exchange:         types.ExchangeBacktest,
		Status:           types.OrderStatusNew,
		ExecutedQuantity: fixedpoint.Zero,
		IsWorking:        true,
		CreationTime:     types.Time(m.currentTime),
		UpdateTime:       types.Time(m.currentTime),
	}
}

func isTakerOrder(o types.Order) bool {
	if o.AveragePrice.IsZero() {
		return false
	}

	switch o.Side {
	case types.SideTypeBuy:
		return o.AveragePrice.Compare(o.Price) < 0

	case types.SideTypeSell:
		return o.AveragePrice.Compare(o.Price) > 0

	}
	return false
}

func isLimitTakerOrder(o types.SubmitOrder, currentPrice fixedpoint.Value) bool {
	if currentPrice.IsZero() {
		return false
	}

	return o.Type == types.OrderTypeLimit && ((o.Side == types.SideTypeBuy && o.Price.Compare(currentPrice) >= 0) ||
		(o.Side == types.SideTypeSell && o.Price.Compare(currentPrice) <= 0))
}
