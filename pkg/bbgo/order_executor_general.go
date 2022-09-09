package bbgo

import (
	"context"
	"errors"
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type NotifyFunc func(obj interface{}, args ...interface{})

// GeneralOrderExecutor implements the general order executor for strategy
type GeneralOrderExecutor struct {
	session            *ExchangeSession
	symbol             string
	strategy           string
	strategyInstanceID string
	position           *types.Position
	activeMakerOrders  *ActiveOrderBook
	orderStore         *OrderStore
	tradeCollector     *TradeCollector
}

func NewGeneralOrderExecutor(session *ExchangeSession, symbol, strategy, strategyInstanceID string, position *types.Position) *GeneralOrderExecutor {
	// Always update the position fields
	position.Strategy = strategy
	position.StrategyInstanceID = strategyInstanceID

	orderStore := NewOrderStore(symbol)
	return &GeneralOrderExecutor{
		session:            session,
		symbol:             symbol,
		strategy:           strategy,
		strategyInstanceID: strategyInstanceID,
		position:           position,
		activeMakerOrders:  NewActiveOrderBook(symbol),
		orderStore:         orderStore,
		tradeCollector:     NewTradeCollector(symbol, position, orderStore),
	}
}

func (e *GeneralOrderExecutor) ActiveMakerOrders() *ActiveOrderBook {
	return e.activeMakerOrders
}

func (e *GeneralOrderExecutor) BindEnvironment(environ *Environment) {
	e.tradeCollector.OnProfit(func(trade types.Trade, profit *types.Profit) {
		environ.RecordPosition(e.position, trade, profit)
	})
}

func (e *GeneralOrderExecutor) BindTradeStats(tradeStats *types.TradeStats) {
	e.tradeCollector.OnProfit(func(trade types.Trade, profit *types.Profit) {
		if profit == nil {
			return
		}

		tradeStats.Add(profit)
	})
}

func (e *GeneralOrderExecutor) BindProfitStats(profitStats *types.ProfitStats) {
	e.tradeCollector.OnProfit(func(trade types.Trade, profit *types.Profit) {
		profitStats.AddTrade(trade)
		if profit == nil {
			return
		}

		profitStats.AddProfit(*profit)

		Notify(profit)
		Notify(profitStats)
	})
}

func (e *GeneralOrderExecutor) Bind() {
	e.activeMakerOrders.BindStream(e.session.UserDataStream)
	e.orderStore.BindStream(e.session.UserDataStream)

	// trade notify
	e.tradeCollector.OnTrade(func(trade types.Trade, profit, netProfit fixedpoint.Value) {
		Notify(trade)
	})

	e.tradeCollector.OnPositionUpdate(func(position *types.Position) {
		log.Infof("position changed: %s", position)
		Notify(position)
	})

	e.tradeCollector.BindStream(e.session.UserDataStream)
}

// CancelOrders cancels the given order objects directly
func (e *GeneralOrderExecutor) CancelOrders(ctx context.Context, orders ...types.Order) error {
	err := e.session.Exchange.CancelOrders(ctx, orders...)
	if err != nil { // Retry once
		err = e.session.Exchange.CancelOrders(ctx, orders...)
	}
	return err
}

func (e *GeneralOrderExecutor) SubmitOrders(ctx context.Context, submitOrders ...types.SubmitOrder) (types.OrderSlice, error) {
	formattedOrders, err := e.session.FormatOrders(submitOrders)
	if err != nil {
		return nil, err
	}

	createdOrders, err := e.session.Exchange.SubmitOrders(ctx, formattedOrders...)
	if err != nil {
		// Retry once
		createdOrders, err = e.session.Exchange.SubmitOrders(ctx, formattedOrders...)
		if err != nil {
			err = fmt.Errorf("can not place orders: %w", err)
		}
	}
	// FIXME: map by price and volume
	for i := 0; i < len(createdOrders); i++ {
		createdOrders[i].Tag = formattedOrders[i].Tag
	}

	e.orderStore.Add(createdOrders...)
	e.activeMakerOrders.Add(createdOrders...)
	e.tradeCollector.Process()
	return createdOrders, err
}

type OpenPositionOptions struct {
	// Long is for open a long position
	// Long or Short must be set
	Long bool

	// Short is for open a short position
	// Long or Short must be set
	Short bool

	// Leverage is used for leveraged position and account
	Leverage fixedpoint.Value

	Quantity        fixedpoint.Value
	MarketOrder     bool
	LimitOrder      bool
	LimitTakerRatio fixedpoint.Value
	CurrentPrice    fixedpoint.Value
	Tag             string
}

func (e *GeneralOrderExecutor) OpenPosition(ctx context.Context, options OpenPositionOptions) error {
	price := options.CurrentPrice
	submitOrder := types.SubmitOrder{
		Symbol:           e.position.Symbol,
		Type:             types.OrderTypeMarket,
		MarginSideEffect: types.SideEffectTypeMarginBuy,
		Tag:              options.Tag,
	}

	if !options.LimitTakerRatio.IsZero() {
		if options.Long {
			// use higher price to buy (this ensures that our order will be filled)
			price = price.Mul(one.Add(options.LimitTakerRatio))
		} else if options.Short {
			// use lower price to sell (this ensures that our order will be filled)
			price = price.Mul(one.Sub(options.LimitTakerRatio))
		}
	}

	if options.MarketOrder {
		submitOrder.Type = types.OrderTypeMarket
	} else if options.LimitOrder {
		submitOrder.Type = types.OrderTypeLimit
		submitOrder.Price = price
	}

	quantity := options.Quantity

	if options.Long {
		if quantity.IsZero() {
			quoteQuantity, err := CalculateQuoteQuantity(ctx, e.session, e.position.QuoteCurrency, options.Leverage)
			if err != nil {
				return err
			}

			quantity = quoteQuantity.Div(price)
		}

		submitOrder.Side = types.SideTypeBuy
		submitOrder.Quantity = quantity

		createdOrder, err2 := e.SubmitOrders(ctx, submitOrder)
		if err2 != nil {
			return err2
		}
		_ = createdOrder
		return nil
	} else if options.Short {
		if quantity.IsZero() {
			var err error
			quantity, err = CalculateBaseQuantity(e.session, e.position.Market, price, quantity, options.Leverage)
			if err != nil {
				return err
			}
		}

		submitOrder.Side = types.SideTypeSell
		submitOrder.Quantity = quantity

		createdOrder, err2 := e.SubmitOrders(ctx, submitOrder)
		if err2 != nil {
			return err2
		}
		_ = createdOrder
		return nil
	}

	return errors.New("options Long or Short must be set")
}

// GracefulCancelActiveOrderBook cancels the orders from the active orderbook.
func (e *GeneralOrderExecutor) GracefulCancelActiveOrderBook(ctx context.Context, activeOrders *ActiveOrderBook) error {
	if activeOrders.NumOfOrders() == 0 {
		return nil
	}
	if err := activeOrders.GracefulCancel(ctx, e.session.Exchange); err != nil {
		// Retry once
		if err = activeOrders.GracefulCancel(ctx, e.session.Exchange); err != nil {
			return fmt.Errorf("graceful cancel order error: %w", err)
		}
	}

	e.tradeCollector.Process()
	return nil
}

func (e *GeneralOrderExecutor) GracefulCancelOrder(ctx context.Context, order types.Order) error {
	if e.activeMakerOrders.NumOfOrders() == 0 {
		return nil
	}
	if err := e.activeMakerOrders.Cancel(ctx, e.session.Exchange, order); err != nil {
		// Retry once
		if err = e.activeMakerOrders.Cancel(ctx, e.session.Exchange, order); err != nil {
			return fmt.Errorf("cancel order error: %w", err)
		}
	}
	e.tradeCollector.Process()
	return nil
}

// GracefulCancel cancels all active maker orders
func (e *GeneralOrderExecutor) GracefulCancel(ctx context.Context) error {
	return e.GracefulCancelActiveOrderBook(ctx, e.activeMakerOrders)
}

func (e *GeneralOrderExecutor) ClosePosition(ctx context.Context, percentage fixedpoint.Value, tags ...string) error {
	submitOrder := e.position.NewMarketCloseOrder(percentage)
	if submitOrder == nil {
		return nil
	}

	log.Infof("closing %s position with tags: %v", e.symbol, tags)
	submitOrder.Tag = strings.Join(tags, ",")
	_, err := e.SubmitOrders(ctx, *submitOrder)
	return err
}

func (e *GeneralOrderExecutor) TradeCollector() *TradeCollector {
	return e.tradeCollector
}

func (e *GeneralOrderExecutor) Session() *ExchangeSession {
	return e.session
}

func (e *GeneralOrderExecutor) Position() *types.Position {
	return e.position
}
