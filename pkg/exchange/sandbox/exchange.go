package sandbox

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type Stream struct {
	types.StandardStream
}

func (s *Stream) Connect(ctx context.Context) error {
	s.StandardStream.EmitConnect()
	return nil
}

func (s *Stream) Close() error {
	return nil
}

type Exchange struct {
	publicEx types.ExchangePublic

	marketDataStream types.Stream
	userDataStream   *Stream

	account *types.Account

	orderId uint64

	tradeId uint64

	openOrders map[uint64]types.Order

	markets types.MarketMap

	mu sync.Mutex

	// trading symbol
	symbol string
}

func New(publicEx types.ExchangePublic, marketDataStream types.Stream, symbol string, bals types.BalanceMap) *Exchange {
	stream := &Stream{
		StandardStream: types.NewStandardStream(),
	}

	stream.OnConnect(func() {
		go stream.EmitAuth()
	})

	account := types.NewAccount()
	account.UpdateBalances(bals)

	ex := &Exchange{
		publicEx:         publicEx,
		marketDataStream: marketDataStream,
		userDataStream:   stream,
		account:          account,
		symbol:           symbol,
		openOrders:       make(map[uint64]types.Order),
	}

	marketDataStream.OnKLine(types.KLineWith(symbol, types.Interval1m, func(k types.KLine) {
		ex.matchOpenOrders(k.Close)
	}))

	return ex
}

// matchOpenOrders matches open orders with the latest close price
func (e *Exchange) matchOpenOrders(closePrice fixedpoint.Value) {
	for orderID := range e.openOrders {
		e.mu.Lock()
		order := e.openOrders[orderID]
		e.mu.Unlock()

		if order.Type != types.OrderTypeLimit && order.Type != types.OrderTypeLimitMaker {
			continue
		}

		matched := false
		switch order.Side {
		case types.SideTypeBuy:
			if order.Price.Compare(closePrice) >= 0 {
				matched = true
			}
		case types.SideTypeSell:
			if order.Price.Compare(closePrice) <= 0 {
				matched = true
			}
		}

		if !matched {
			continue
		}

		order.Status = types.OrderStatusFilled
		order.ExecutedQuantity = order.Quantity
		order.AveragePrice = closePrice

		trade := newMockTradeFromOrder(&order, true)
		e.userDataStream.EmitTradeUpdate(trade)
		e.userDataStream.EmitOrderUpdate(order)

		balances := e.account.Balances()
		switch order.Side {
		case types.SideTypeBuy:
			quoteBal := balances[order.Market.QuoteCurrency]
			baseBal := balances[order.Market.BaseCurrency]
			cost := order.Quantity.Mul(order.Price)
			quoteBal.Locked = quoteBal.Locked.Sub(cost)
			baseBal.Available = baseBal.Available.Add(order.Quantity)
			balances[order.Market.QuoteCurrency] = quoteBal
			balances[order.Market.BaseCurrency] = baseBal
		case types.SideTypeSell:
			baseBal := balances[order.Market.BaseCurrency]
			quoteBal := balances[order.Market.QuoteCurrency]
			baseBal.Locked = baseBal.Locked.Sub(order.Quantity)
			quoteBal.Available = quoteBal.Available.Add(order.Quantity.Mul(order.Price))
			balances[order.Market.BaseCurrency] = baseBal
			balances[order.Market.QuoteCurrency] = quoteBal
		}

		e.account.UpdateBalances(balances)
		e.userDataStream.EmitBalanceUpdate(balances)

		e.mu.Lock()
		delete(e.openOrders, orderID)
		e.mu.Unlock()
	}
}

func (e *Exchange) Initialize(ctx context.Context) error {
	markets, err := e.publicEx.QueryMarkets(ctx)
	if err != nil {
		return err
	}

	e.markets = markets
	return nil
}

func (e *Exchange) Name() types.ExchangeName {
	return e.publicEx.Name()
}

func (e *Exchange) PlatformFeeCurrency() string {
	return e.publicEx.PlatformFeeCurrency()
}

func (e *Exchange) NewStream() types.Stream {
	return e.publicEx.NewStream()
}

func (e *Exchange) GetUserDataStream() types.Stream {
	return e.userDataStream
}

func (e *Exchange) QueryMarkets(ctx context.Context) (types.MarketMap, error) {
	return e.publicEx.QueryMarkets(ctx)
}

func (e *Exchange) QueryTicker(ctx context.Context, symbol string) (*types.Ticker, error) {
	return e.publicEx.QueryTicker(ctx, symbol)
}

func (e *Exchange) QueryTickers(ctx context.Context, symbol ...string) (map[string]types.Ticker, error) {
	return e.publicEx.QueryTickers(ctx, symbol...)
}

func (e *Exchange) QueryKLines(
	ctx context.Context, symbol string, interval types.Interval, options types.KLineQueryOptions,
) ([]types.KLine, error) {
	return e.publicEx.QueryKLines(ctx, symbol, interval, options)
}

func (e *Exchange) QueryAccount(ctx context.Context) (*types.Account, error) {
	return e.account, nil
}

func (e *Exchange) QueryAccountBalances(ctx context.Context) (types.BalanceMap, error) {
	return e.account.Balances(), nil
}

// SubmitOrder simulates order submission
func (e *Exchange) SubmitOrder(ctx context.Context, order types.SubmitOrder) (createdOrder *types.Order, err error) {
	log.Infof("SubmitOrder: %+v", order)

	e.mu.Lock()
	defer e.mu.Unlock()

	market, ok := e.markets[order.Symbol]
	if !ok {
		return nil, fmt.Errorf("market not found: %s", order.Symbol)
	}

	var emptyMarket types.Market
	if order.Market == emptyMarket {
		order.Market = market
	}

	ticker, err := e.publicEx.QueryTicker(ctx, order.Symbol)
	if err != nil {
		return nil, err
	}

	// balance check and deduction/lock
	balances := e.account.Balances()
	switch order.Side {
	case types.SideTypeBuy:
		cost := order.Quantity.Mul(order.Price)
		quoteBal, ok := balances[market.QuoteCurrency]
		if !ok || quoteBal.Available.Compare(cost) < 0 {
			return nil, fmt.Errorf("insufficient quote balance: need %s %s, available %s", cost.String(), market.QuoteCurrency, quoteBal.Available.String())
		}
		switch order.Type {
		case types.OrderTypeMarket:
			quoteBal.Available = quoteBal.Available.Sub(cost)
		case types.OrderTypeLimit, types.OrderTypeLimitMaker, types.OrderTypeStopLimit, types.OrderTypeStopMarket:
			quoteBal.Available = quoteBal.Available.Sub(cost)
			quoteBal.Locked = quoteBal.Locked.Add(cost)
		default:
			quoteBal.Available = quoteBal.Available.Sub(cost)
			quoteBal.Locked = quoteBal.Locked.Add(cost)
		}
		balances[market.QuoteCurrency] = quoteBal
	case types.SideTypeSell:
		baseBal, ok := balances[market.BaseCurrency]
		if !ok || baseBal.Available.Compare(order.Quantity) < 0 {
			return nil, fmt.Errorf("insufficient base balance: need %s %s, available %s", order.Quantity.String(), market.BaseCurrency, baseBal.Available.String())
		}
		switch order.Type {
		case types.OrderTypeMarket:
			baseBal.Available = baseBal.Available.Sub(order.Quantity)
		case types.OrderTypeLimit, types.OrderTypeLimitMaker, types.OrderTypeStopLimit, types.OrderTypeStopMarket:
			baseBal.Available = baseBal.Available.Sub(order.Quantity)
			baseBal.Locked = baseBal.Locked.Add(order.Quantity)
		default:
			baseBal.Available = baseBal.Available.Sub(order.Quantity)
			baseBal.Locked = baseBal.Locked.Add(order.Quantity)
		}
		balances[market.BaseCurrency] = baseBal
	}

	e.account.UpdateBalances(balances)
	e.userDataStream.EmitBalanceUpdate(balances)

	orderId := atomic.AddUint64(&e.orderId, 1)
	createdOrder = &types.Order{
		SubmitOrder: order,
		OrderID:     orderId,
		Status:      types.OrderStatusNew,
	}

	switch order.Type {
	case types.OrderTypeMarket:
		createdOrder.Status = types.OrderStatusFilled
		createdOrder.ExecutedQuantity = order.Quantity

		switch order.Side {
		case types.SideTypeBuy:
			createdOrder.AveragePrice = ticker.Sell
		case types.SideTypeSell:
			createdOrder.AveragePrice = ticker.Buy
		}

		trade := newMockTradeFromOrder(createdOrder, false)
		e.userDataStream.EmitTradeUpdate(trade)
		e.userDataStream.EmitOrderUpdate(*createdOrder)

		return createdOrder, nil
	}

	e.openOrders[e.orderId] = *createdOrder
	return createdOrder, nil
}

func (e *Exchange) QueryOpenOrders(ctx context.Context, symbol string) (orders []types.Order, err error) {
	log.Info("QueryOpenOrders")

	e.mu.Lock()
	defer e.mu.Unlock()

	orders = make([]types.Order, 0)
	for _, order := range e.openOrders {
		if order.Symbol == symbol {
			orders = append(orders, order)
		}
	}

	return orders, nil
}

func (e *Exchange) CancelOrders(ctx context.Context, orders ...types.Order) error {
	log.Info("CancelOrders")

	for _, order := range orders {
		e.mu.Lock()
		_, ok := e.openOrders[order.OrderID]
		e.mu.Unlock()

		if !ok {
			return fmt.Errorf("order not found: %d", order.OrderID)
		}

		market, ok := e.markets[order.Symbol]
		if !ok {
			return fmt.Errorf("market not found: %s", order.Symbol)
		}

		balances := e.account.Balances()
		switch order.Side {
		case types.SideTypeBuy:
			bal, ok := balances[market.QuoteCurrency]
			if !ok {
				return fmt.Errorf("quote balance not found: %s", market.QuoteCurrency)
			}
			bal.Available = bal.Available.Add(order.Quantity.Mul(order.Price))
			bal.Locked = bal.Locked.Sub(order.Quantity.Mul(order.Price))
			balances[market.QuoteCurrency] = bal
		case types.SideTypeSell:
			bal, ok := balances[market.BaseCurrency]
			if !ok {
				return fmt.Errorf("base balance not found: %s", market.BaseCurrency)
			}
			bal.Available = bal.Available.Add(order.Quantity)
			bal.Locked = bal.Locked.Sub(order.Quantity)
			balances[market.BaseCurrency] = bal
		default:
			return fmt.Errorf("unknown order side: %s", order.Side)
		}

		e.account.UpdateBalances(balances)
		e.userDataStream.EmitBalanceUpdate(balances)

		order.Status = types.OrderStatusCanceled
		e.userDataStream.EmitOrderUpdate(order)

		e.mu.Lock()
		delete(e.openOrders, order.OrderID)
		e.mu.Unlock()
	}

	return nil
}

// NewMockTradeFromOrder creates a mock trade from the given order
func newMockTradeFromOrder(order *types.Order, isMaker bool) types.Trade {
	price := order.Price

	if price.IsZero() && order.AveragePrice.Sign() > 0 {
		price = order.AveragePrice
	}

	return types.Trade{
		ID:            order.OrderID,
		OrderID:       order.OrderID,
		Exchange:      order.Exchange,
		Price:         price,
		Quantity:      order.Quantity,
		QuoteQuantity: order.Price.Mul(order.Quantity),
		Symbol:        order.Symbol,
		Side:          order.Side,
		IsBuyer:       order.Side == types.SideTypeBuy,
		IsMaker:       isMaker,
		Time:          types.Time(time.Now()),
		Fee:           fixedpoint.Zero,
		FeeCurrency:   order.Market.QuoteCurrency,
	}
}
