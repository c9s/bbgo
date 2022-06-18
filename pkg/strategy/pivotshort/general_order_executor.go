package pivotshort

import (
	"context"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

// GeneralOrderExecutor implements the general order executor for strategy
type GeneralOrderExecutor struct {
	session            *bbgo.ExchangeSession
	symbol             string
	strategy           string
	strategyInstanceID string
	position           *types.Position
	activeMakerOrders  *bbgo.ActiveOrderBook
	orderStore         *bbgo.OrderStore
	tradeCollector     *bbgo.TradeCollector
}

func NewGeneralOrderExecutor(session *bbgo.ExchangeSession, symbol, strategy, strategyInstanceID string, position *types.Position) *GeneralOrderExecutor {
	// Always update the position fields
	position.Strategy = strategy
	position.StrategyInstanceID = strategyInstanceID

	orderStore := bbgo.NewOrderStore(symbol)
	return &GeneralOrderExecutor{
		session:            session,
		symbol:             symbol,
		strategy:           strategy,
		strategyInstanceID: strategyInstanceID,
		position:           position,
		activeMakerOrders:  bbgo.NewActiveOrderBook(symbol),
		orderStore:         orderStore,
		tradeCollector:     bbgo.NewTradeCollector(symbol, position, orderStore),
	}
}

func (e *GeneralOrderExecutor) BindProfitStats(profitStats *types.ProfitStats, notify func(obj interface{}, args ...interface{})) {
	// profit stats
	e.tradeCollector.OnProfit(func(trade types.Trade, profit *types.Profit) {
		profitStats.AddTrade(trade)
		if profit == nil {
			return
		}

		profitStats.AddProfit(*profit)
		notify(&profitStats)
	})
}

func (e *GeneralOrderExecutor) Bind(notify func(obj interface{}, args ...interface{})) {
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
	submitOrder := e.position.NewMarketCloseOrder(percentage) // types.SubmitOrder{
	if submitOrder == nil {
		return nil
	}

	return e.SubmitOrders(ctx, *submitOrder)
}
