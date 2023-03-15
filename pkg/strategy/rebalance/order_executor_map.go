package rebalance

import (
	"context"
	"fmt"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/types"
)

type GeneralOrderExecutorMap map[string]*bbgo.GeneralOrderExecutor

func NewGeneralOrderExecutorMap(session *bbgo.ExchangeSession, positionMap PositionMap) GeneralOrderExecutorMap {
	m := make(GeneralOrderExecutorMap)

	for symbol, position := range positionMap {
		log.Infof("creating order executor for symbol %s", symbol)
		orderExecutor := bbgo.NewGeneralOrderExecutor(session, symbol, ID, instanceID(symbol), position)
		m[symbol] = orderExecutor
	}

	return m
}

func (m GeneralOrderExecutorMap) BindEnvironment(environ *bbgo.Environment) {
	for _, orderExecutor := range m {
		orderExecutor.BindEnvironment(environ)
	}
}

func (m GeneralOrderExecutorMap) BindProfitStats(profitStatsMap ProfitStatsMap) {
	for symbol, orderExecutor := range m {
		log.Infof("binding profit stats for symbol %s", symbol)
		orderExecutor.BindProfitStats(profitStatsMap[symbol])
	}
}

func (m GeneralOrderExecutorMap) Bind() {
	for _, orderExecutor := range m {
		orderExecutor.Bind()
	}
}

func (m GeneralOrderExecutorMap) Sync(ctx context.Context, obj interface{}) {
	for _, orderExecutor := range m {
		orderExecutor.TradeCollector().OnPositionUpdate(func(position *types.Position) {
			bbgo.Sync(ctx, obj)
		})
	}
}

func (m GeneralOrderExecutorMap) SubmitOrders(ctx context.Context, submitOrders ...types.SubmitOrder) (types.OrderSlice, error) {
	var allCreatedOrders types.OrderSlice
	for _, submitOrder := range submitOrders {
		log.Infof("submitting order: %+v", submitOrder)
		orderExecutor, ok := m[submitOrder.Symbol]
		if !ok {
			return nil, fmt.Errorf("order executor not found for symbol %s", submitOrder.Symbol)
		}

		createdOrders, err := orderExecutor.SubmitOrders(ctx, submitOrder)
		if err != nil {
			return nil, err
		}
		allCreatedOrders = append(allCreatedOrders, createdOrders...)
	}

	return allCreatedOrders, nil
}

func (m GeneralOrderExecutorMap) GracefulCancel(ctx context.Context) error {
	for _, orderExecutor := range m {
		err := orderExecutor.GracefulCancel(ctx)
		if err != nil {
			return err
		}
	}
	return nil
}
