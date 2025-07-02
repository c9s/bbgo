package tradingdesk

import (
	"context"
	"fmt"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/types"
)

type TradingManager struct {
	Position      *types.Position    `persistence:"position"`
	ProfitStats   *types.ProfitStats `persistence:"profitStats"`
	OrderExecutor *bbgo.GeneralOrderExecutor
}

func (m *TradingManager) Initialize(ctx context.Context, environ *bbgo.Environment, session *bbgo.ExchangeSession, market types.Market, strategyID, instanceID string) {
	if m.Position == nil {
		m.Position = types.NewPositionFromMarket(market)
		m.Position.Strategy = strategyID
		m.Position.StrategyInstanceID = instanceID
	}

	if m.ProfitStats == nil {
		m.ProfitStats = types.NewProfitStats(market)
	}

	m.OrderExecutor = bbgo.NewGeneralOrderExecutor(session, market.Symbol, strategyID, instanceID, m.Position)
	m.OrderExecutor.BindEnvironment(environ)
	m.OrderExecutor.BindProfitStats(m.ProfitStats)
	m.OrderExecutor.Bind()
}

type TradingManagerMap map[string]*TradingManager

func (m TradingManagerMap) GetOrderExecutor(ctx context.Context, environ *bbgo.Environment, session *bbgo.ExchangeSession, symbol, strategyID, instanceID string) (*bbgo.GeneralOrderExecutor, error) {
	market, ok := session.Market(symbol)
	if !ok {
		return nil, fmt.Errorf("market %s not found", symbol)
	}

	manager, ok := m[symbol]
	if !ok {
		manager = &TradingManager{}
		m[symbol] = manager
	}

	manager.Initialize(ctx, environ, session, market, strategyID, instanceID)
	return manager.OrderExecutor, nil
}
