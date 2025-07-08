package tradingdesk

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/service"
	"github.com/c9s/bbgo/pkg/types"
)

const ID = "tradingdesk"

var log = logrus.WithField("strategy", ID)

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})

	_ = bbgo.CustomSync(&Strategy{})
}

type PositionMap map[string]*types.Position
type ProfitStatsMap map[string]*types.ProfitStats

type Strategy struct {
	Environment *bbgo.Environment

	session *bbgo.ExchangeSession

	MaxLossLimit fixedpoint.Value `json:"maxLossLimit"`
	PriceType    types.PriceType  `json:"priceType"`

	TestOpenPosition *OpenPositionParams `json:"testOpenPosition"`

	tradingManagers TradingManagerMap
	logger          logrus.FieldLogger

	riskService types.ExchangeRiskService
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) InstanceID() string {
	return ID
}

func (s *Strategy) Load(ctx context.Context, store service.Store) error {
	snapshot := make(map[string]TradingManagerState)

	if err := store.Load(&snapshot); err != nil {
		if errors.Is(err, service.ErrPersistenceNotExists) {
			return nil
		}

		return err
	}

	if s.tradingManagers == nil {
		s.tradingManagers = make(TradingManagerMap)
	}

	for symbol, state := range snapshot {
		s.tradingManagers[symbol] = &TradingManager{
			TradingManagerState: state,
		}
	}

	return nil
}

func (s *Strategy) Store(ctx context.Context, store service.Store) error {
	snapshot := make(map[string]TradingManagerState)
	for symbol, manager := range s.tradingManagers {
		snapshot[symbol] = manager.TradingManagerState
	}

	return store.Save(snapshot)
}

func (s *Strategy) Initialize() error {
	if s.tradingManagers == nil {
		s.tradingManagers = make(TradingManagerMap)
	}

	s.logger = logrus.WithField("strategy", ID)
	return nil
}

func (s *Strategy) Defaults() error {
	if s.PriceType == "" {
		s.PriceType = types.PriceTypeMaker
	}

	return nil
}

func (s *Strategy) Validate() error {
	if s.MaxLossLimit.Sign() <= 0 {
		return fmt.Errorf("maxLossLimit should be greater than zero")
	}

	return nil
}

// Subscribe implements the strategy interface for market data subscriptions
func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	// Currently no market data subscriptions needed
}

// Run implements the strategy interface for strategy execution
func (s *Strategy) Run(ctx context.Context, _ bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	s.session = session

	if riskService, ok := session.Exchange.(types.ExchangeRiskService); ok {
		s.riskService = riskService
	}

	if s.TestOpenPosition != nil {
		if session.Futures && s.riskService != nil {
			s.logger.Infof("querying position risk for %s", s.TestOpenPosition.Symbol)

			risks, err := s.riskService.QueryPositionRisk(ctx)
			if err != nil {
				return err
			}

			s.logger.Infof("got position risks: %+v", risks)
			for _, risk := range risks {
				market, _ := session.Market(risk.Symbol)

				m, ok := s.tradingManagers[risk.Symbol]
				if !ok {
					m = s.tradingManagers.Get(ctx, s.Environment, s.session, market, s)
				}

				m.Position.Symbol = risk.Symbol
				m.Position.Base = risk.PositionAmount

				switch risk.PositionSide {
				case types.PositionLong:
				case types.PositionShort:
					m.Position.Base = m.Position.Base.Neg()
				}
			}
		}

		if err := s.OpenPosition(ctx, *s.TestOpenPosition); err != nil {
			return fmt.Errorf("open position error: %w", err)
		}
	}

	bbgo.OnShutdown(ctx, func(ctx context.Context, wg *sync.WaitGroup) {
		defer wg.Done()

		if s.TestOpenPosition != nil {
			param := s.TestOpenPosition
			market, ok := s.session.Market(param.Symbol)
			if !ok {
				s.logger.Warnf("market %s not found in session, unable to close position", param.Symbol)
				return
			}

			m := s.tradingManagers.Get(ctx, s.Environment, s.session, market, s)
			if err := m.ClosePosition(ctx); err != nil {
				s.logger.WithError(err).Errorf("unable to close position for symbol %s", param.Symbol)
			}

			time.Sleep(1 * time.Second)
		}
	})

	return nil
}

// OpenPosition opens a new position with risk-based position sizing.
// The position size is calculated based on MaxLossLimit, stop loss price, and available balance.
func (s *Strategy) OpenPosition(ctx context.Context, param OpenPositionParams) error {
	market, ok := s.session.Market(param.Symbol)
	if !ok {
		return fmt.Errorf("market %s not found in session", param.Symbol)
	}

	m := s.tradingManagers.Get(ctx, s.Environment, s.session, market, s)
	return m.OpenPosition(ctx, param)
}
