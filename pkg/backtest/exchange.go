package backtest

import (
	"context"
	"time"

	"github.com/pkg/errors"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/exchange/binance"
	"github.com/c9s/bbgo/pkg/exchange/max"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/service"
	"github.com/c9s/bbgo/pkg/types"
)

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

type Exchange struct {
	sourceExchange types.ExchangeName
	publicExchange types.Exchange
	srv            *service.BacktestService
	startTime      time.Time

	account *types.Account
	config  *bbgo.Backtest

	closedOrders []types.SubmitOrder
	openOrders   []types.SubmitOrder

	stream *Stream
}

func NewExchange(sourceExchange types.ExchangeName, srv *service.BacktestService, config *bbgo.Backtest) *Exchange {
	ex, err := newPublicExchange(sourceExchange)
	if err != nil {
		panic(err)
	}

	if config == nil {
		panic(errors.New("backtest config can not be nil"))
	}

	startTime, err := config.ParseStartTime()
	if err != nil {
		panic(err)
	}

	balances := config.Account.Balances.BalanceMap()

	account := &types.Account{
		MakerCommission: config.Account.MakerCommission,
		TakerCommission: config.Account.TakerCommission,
		AccountType:     "SPOT", // currently not used
	}
	account.UpdateBalances(balances)

	return &Exchange{
		sourceExchange: sourceExchange,
		publicExchange: ex,
		srv:            srv,
		config:         config,
		account:        account,
		startTime:      startTime,
	}
}

func (e *Exchange) NewStream() types.Stream {
	if e.stream != nil {
		panic("backtest stream is already allocated, please check if there are extra NewStream calls")
	}

	e.stream = &Stream{exchange: e}
	return e.stream
}

func (e Exchange) SubmitOrders(ctx context.Context, orders ...types.SubmitOrder) (createdOrders types.OrderSlice, err error) {
	panic("implement me")
}

func (e Exchange) QueryOpenOrders(ctx context.Context, symbol string) (orders []types.Order, err error) {
	panic("implement me")
}

func (e Exchange) QueryClosedOrders(ctx context.Context, symbol string, since, until time.Time, lastOrderID uint64) (orders []types.Order, err error) {
	panic("implement me")
}

func (e Exchange) CancelOrders(ctx context.Context, orders ...types.Order) error {
	panic("implement me")
}

func (e Exchange) QueryAccount(ctx context.Context) (*types.Account, error) {
	return e.account, nil
}

func (e *Exchange) QueryAccountBalances(ctx context.Context) (types.BalanceMap, error) {
	return e.account.Balances(), nil
}

func (e Exchange) QueryKLines(ctx context.Context, symbol string, interval types.Interval, options types.KLineQueryOptions) ([]types.KLine, error) {
	return e.publicExchange.QueryKLines(ctx, symbol, interval, options)
}

func (e Exchange) QueryTrades(ctx context.Context, symbol string, options *types.TradeQueryOptions) ([]types.Trade, error) {
	// we don't need query trades for backtest
	return nil, nil
}

func (e Exchange) Name() types.ExchangeName {
	return e.publicExchange.Name()
}

func (e Exchange) PlatformFeeCurrency() string {
	return e.publicExchange.PlatformFeeCurrency()
}

func (e Exchange) QueryMarkets(ctx context.Context) (types.MarketMap, error) {
	return e.publicExchange.QueryMarkets(ctx)
}

func (e Exchange) QueryDepositHistory(ctx context.Context, asset string, since, until time.Time) (allDeposits []types.Deposit, err error) {
	return nil, nil
}

func (e Exchange) QueryWithdrawHistory(ctx context.Context, asset string, since, until time.Time) (allWithdraws []types.Withdraw, err error) {
	return nil, nil
}

func newPublicExchange(sourceExchange types.ExchangeName) (types.Exchange, error) {
	switch sourceExchange {
	case types.ExchangeBinance:
		return binance.New("", ""), nil
	case types.ExchangeMax:
		return max.New("", ""), nil
	}

	return nil, errors.Errorf("exchange %s is not supported", sourceExchange)
}
