package tradingdesk

import (
	"context"

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

	TradingManagerMap TradingManagerMap `persistence:"tradingManagerMap"`

	MaxLossLimit fixedpoint.Value `json:"maxLossLimit"`
	PriceType    types.PriceType  `json:"priceType"`
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) InstanceID() string {
	return ID
}

func (s *Strategy) Initialize() error {
	if s.TradingManagerMap == nil {
		s.TradingManagerMap = make(TradingManagerMap)
	}
	return nil
}

func (s *Strategy) Validate() error {
	if s.PriceType == "" {
		s.PriceType = types.PriceTypeMaker
	}
	return nil
}

// Subscribe implements the strategy interface for market data subscriptions
func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	// Currently no market data subscriptions needed
}

// Run implements the strategy interface for strategy execution
func (s *Strategy) Run(ctx context.Context, _ bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	s.Session = session
	// Strategy is ready to receive OpenPosition calls
	return nil
}

// OpenPosition opens a new position with risk-based position sizing.
// The position size is calculated based on MaxLossLimit, stop loss price, and available balance.
func (s *Strategy) OpenPosition(ctx context.Context, param OpenPositionParam) error {
	m, err := s.TradingManagerMap.GetTradingManager(ctx, s.Environment, s.Session, param.Symbol, s.ID(), s.InstanceID())
	if err != nil {
		return err
	}

	// Configure the trading manager with strategy parameters
	m.MaxLossLimit = s.MaxLossLimit
	m.PriceType = s.PriceType

	// Delegate to the trading manager
	return m.OpenPosition(ctx, param)
}
