package tradingdesk

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

const ID = "tradingdesk"

var log = logrus.WithField("strategy", ID)

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

type PositionMap map[string]*types.Position
type ProfitStatsMap map[string]*types.ProfitStats

type Strategy struct {
	Session     *bbgo.ExchangeSession
	Environment *bbgo.Environment

	OrderExecutorMap map[string]*bbgo.GeneralOrderExecutor
	PositionMap      PositionMap    `persistence:"position_map"`
	ProfitStatsMap   ProfitStatsMap `persistence:"profit_stats_map"`

	MaxLossLimit fixedpoint.Value `json:"maxLossLimit"`
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) InstanceID() string {
	return ID
}

func (s *Strategy) Initialize() error {
	if s.PositionMap == nil {
		s.PositionMap = make(PositionMap)
	}

	if s.ProfitStatsMap == nil {
		s.ProfitStatsMap = make(ProfitStatsMap)
	}
	return nil
}

func (s *Strategy) Validate() error {
	return nil
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {}

func (s *Strategy) Run(ctx context.Context, _ bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	s.Session = session
	return nil
}

func (s *Strategy) getOrCreatePosition(symbol string) (*types.Position, error) {
	market, ok := s.Session.Market(symbol)
	if !ok {
		return nil, fmt.Errorf("market %s not found", symbol)
	}

	position, ok := s.PositionMap[symbol]
	if !ok {
		position = types.NewPositionFromMarket(market)
		position.Strategy = ID
		position.StrategyInstanceID = s.InstanceID()
		s.PositionMap[symbol] = position
	}

	return position, nil
}

func (s *Strategy) getOrCreateProfitStats(symbol string) (*types.ProfitStats, error) {
	market, ok := s.Session.Market(symbol)
	if !ok {
		return nil, fmt.Errorf("market %s not found", symbol)
	}

	profitStats, ok := s.ProfitStatsMap[symbol]
	if !ok {
		profitStats = types.NewProfitStats(market)
		s.ProfitStatsMap[symbol] = profitStats
	}

	return profitStats, nil
}

func (s *Strategy) getOrCreateOrderExecutor(symbol string) (*bbgo.GeneralOrderExecutor, error) {
	if s.OrderExecutorMap == nil {
		s.OrderExecutorMap = make(map[string]*bbgo.GeneralOrderExecutor)
	}

	executor, ok := s.OrderExecutorMap[symbol]
	if ok {
		return executor, nil
	}

	position, err := s.getOrCreatePosition(symbol)
	if err != nil {
		return nil, err
	}

	profitStats, err := s.getOrCreateProfitStats(symbol)
	if err != nil {
		return nil, err
	}

	executor = bbgo.NewGeneralOrderExecutor(s.Session, symbol, s.ID(), s.InstanceID(), position)
	executor.BindEnvironment(s.Environment)
	executor.BindProfitStats(profitStats)
	executor.Bind()

	s.OrderExecutorMap[symbol] = executor
	return executor, nil
}

func (s *Strategy) OpenPosition(ctx context.Context, param OpenPositionParam) error {
	executor, err := s.getOrCreateOrderExecutor(param.Symbol)
	if err != nil {
		return err
	}

	order := types.SubmitOrder{
		Symbol:   param.Symbol,
		Side:     param.Side,
		Type:     types.OrderTypeMarket,
		Quantity: param.Quantity,
	}

	createdOrders, err := executor.SubmitOrders(ctx, order)
	if err != nil {
		log.WithError(err).Errorf("failed to submit market order: %+v", order)
		return err
	}
	log.Infof("created orders: %+v", createdOrders)
	return nil
}
