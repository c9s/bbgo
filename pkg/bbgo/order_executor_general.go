package bbgo

import (
	"context"
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
	return e.session.Exchange.CancelOrders(ctx, orders...)
}

func (e *GeneralOrderExecutor) SubmitOrders(ctx context.Context, submitOrders ...types.SubmitOrder) (types.OrderSlice, error) {
	formattedOrders, err := e.session.FormatOrders(submitOrders)
	if err != nil {
		return nil, err
	}

	createdOrders, err := e.session.Exchange.SubmitOrders(ctx, formattedOrders...)
	if err != nil {
		err = fmt.Errorf("can not place orders: %w", err)
	}

	e.orderStore.Add(createdOrders...)
	e.activeMakerOrders.Add(createdOrders...)
	e.tradeCollector.Process()
	return createdOrders, err
}

// GracefulCancelActiveOrderBook cancels the orders from the active orderbook.
func (e *GeneralOrderExecutor) GracefulCancelActiveOrderBook(ctx context.Context, activeOrders *ActiveOrderBook) error {
	if err := activeOrders.GracefulCancel(ctx, e.session.Exchange); err != nil {
		return fmt.Errorf("graceful cancel order error: %w", err)
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
