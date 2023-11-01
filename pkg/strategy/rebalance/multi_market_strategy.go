package rebalance

import (
	"context"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/types"
)

type MultiMarketStrategy struct {
	Environ *bbgo.Environment
	Session *bbgo.ExchangeSession

	PositionMap      PositionMap    `persistence:"positionMap"`
	ProfitStatsMap   ProfitStatsMap `persistence:"profitStatsMap"`
	OrderExecutorMap GeneralOrderExecutorMap

	parent, ctx context.Context
	cancel      context.CancelFunc
}

func (s *MultiMarketStrategy) Initialize(ctx context.Context, environ *bbgo.Environment, session *bbgo.ExchangeSession, markets map[string]types.Market, strategyID string) {
	s.parent = ctx
	s.ctx, s.cancel = context.WithCancel(ctx)

	s.Environ = environ
	s.Session = session

	if s.PositionMap == nil {
		s.PositionMap = make(PositionMap)
	}
	s.PositionMap.CreatePositions(markets)

	if s.ProfitStatsMap == nil {
		s.ProfitStatsMap = make(ProfitStatsMap)
	}
	s.ProfitStatsMap.CreateProfitStats(markets)

	s.OrderExecutorMap = NewGeneralOrderExecutorMap(session, s.PositionMap)
	s.OrderExecutorMap.BindEnvironment(environ)
	s.OrderExecutorMap.BindProfitStats(s.ProfitStatsMap)
	s.OrderExecutorMap.Sync(ctx, s)
	s.OrderExecutorMap.Bind()
}
