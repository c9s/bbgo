package bbgo

import (
	"context"

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
		tradeStats.Add(profit.Profit)
	})
}

func (e *GeneralOrderExecutor) BindProfitStats(profitStats *types.ProfitStats) {
	e.tradeCollector.OnProfit(func(trade types.Trade, profit *types.Profit) {
		profitStats.AddTrade(trade)
		if profit == nil {
			return
		}

		profitStats.AddProfit(*profit)
		Notify(&profitStats)
	})
}

func (e *GeneralOrderExecutor) Bind(notify NotifyFunc) {
	e.activeMakerOrders.BindStream(e.session.UserDataStream)
	e.orderStore.BindStream(e.session.UserDataStream)

	// trade notify
	e.tradeCollector.OnTrade(func(trade types.Trade, profit, netProfit fixedpoint.Value) {
		notify(trade)
	})

	e.tradeCollector.OnPositionUpdate(func(position *types.Position) {
		log.Infof("position changed: %s", position)
		notify(position)
	})

	e.tradeCollector.BindStream(e.session.UserDataStream)
}

func (e *GeneralOrderExecutor) SubmitOrders(ctx context.Context, submitOrders ...types.SubmitOrder) error {
	formattedOrders, err := e.session.FormatOrders(submitOrders)
	if err != nil {
		return err
	}

	createdOrders, err := e.session.Exchange.SubmitOrders(ctx, formattedOrders...)
	if err != nil {
		log.WithError(err).Errorf("can not place orders")
	}

	e.orderStore.Add(createdOrders...)
	e.activeMakerOrders.Add(createdOrders...)
	e.tradeCollector.Process()
	return err
}

func (e *GeneralOrderExecutor) GracefulCancel(ctx context.Context) error {
	if err := e.activeMakerOrders.GracefulCancel(ctx, e.session.Exchange); err != nil {
		log.WithError(err).Errorf("graceful cancel order error")
		return err
	}

	return nil
}

func (e *GeneralOrderExecutor) ClosePosition(ctx context.Context, percentage fixedpoint.Value) error {
	submitOrder := e.position.NewMarketCloseOrder(percentage)
	if submitOrder == nil {
		return nil
	}

	return e.SubmitOrders(ctx, *submitOrder)
}
