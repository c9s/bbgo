package rebalance

import (
	"context"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/types"
)

type PositionMap map[string]*types.Position
type ProfitStatsMap map[string]*types.ProfitStats

type MultiMarketStrategy struct {
	Environ *bbgo.Environment
	Session *bbgo.ExchangeSession

	PositionMap      PositionMap    `persistence:"position_map"`
	ProfitStatsMap   ProfitStatsMap `persistence:"profit_stats_map"`
	OrderExecutorMap GeneralOrderExecutorMap

	parent, ctx context.Context
	cancel      context.CancelFunc
}

func (s *MultiMarketStrategy) Initialize(ctx context.Context, environ *bbgo.Environment, session *bbgo.ExchangeSession, markets map[string]types.Market, strategyID string, instanceID string) {
	s.parent = ctx
	s.ctx, s.cancel = context.WithCancel(ctx)

	s.Environ = environ
	s.Session = session

	// initialize position map
	if s.PositionMap == nil {
		log.Infof("creating position map")
		s.PositionMap = make(PositionMap)
	}
	for symbol, market := range markets {
		if _, ok := s.PositionMap[symbol]; ok {
			continue
		}

		log.Infof("creating position for symbol %s", symbol)
		position := types.NewPositionFromMarket(market)
		position.Strategy = ID
		position.StrategyInstanceID = instanceID
		s.PositionMap[symbol] = position
	}

	// initialize profit stats map
	if s.ProfitStatsMap == nil {
		log.Infof("creating profit stats map")
		s.ProfitStatsMap = make(ProfitStatsMap)
	}
	for symbol, market := range markets {
		if _, ok := s.ProfitStatsMap[symbol]; ok {
			continue
		}

		log.Infof("creating profit stats for symbol %s", symbol)
		s.ProfitStatsMap[symbol] = types.NewProfitStats(market)
	}

	// initialize order executor map
	s.OrderExecutorMap = NewGeneralOrderExecutorMap(session, strategyID, instanceID, s.PositionMap)
	s.OrderExecutorMap.BindEnvironment(environ)
	s.OrderExecutorMap.BindProfitStats(s.ProfitStatsMap)
	s.OrderExecutorMap.Bind()
}
