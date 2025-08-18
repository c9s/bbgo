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

	Leverage     int              `json:"leverage"`
	MaxLossLimit fixedpoint.Value `json:"maxLossLimit"`
	PriceType    types.PriceType  `json:"priceType"`

	OpenPositions []OpenPositionParams `json:"openPositions"`

	ClosePositionsOnShutdown bool `json:"closePositionsOnShutdown"`

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

	if s.Leverage == 0 {
		s.Leverage = 3
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

func (s *Strategy) loadFromPositionRisks(ctx context.Context) error {
	s.logger.Infof("querying position risks...")

	risks, err := s.riskService.QueryPositionRisk(ctx)
	if err != nil {
		return err
	}

	s.logger.Infof("loaded position risks: %+v", risks)
	for _, risk := range risks {
		market, _ := s.session.Market(risk.Symbol)

		m, ok := s.tradingManagers[risk.Symbol]
		if !ok {
			m = s.tradingManagers.Get(ctx, s.Environment, s.session, market, s)
		}

		m.Position.Symbol = risk.Symbol
		m.Position.AverageCost = risk.EntryPrice
		m.Position.Base = risk.PositionAmount
		m.Position.Market = market

		switch risk.PositionSide {
		case types.PositionLong:
		case types.PositionShort:
			m.Position.Base = m.Position.Base.Neg()
		}

		// Query open orders for the symbol
		orders, err := s.session.Exchange.QueryOpenOrders(ctx, risk.Symbol)
		if err != nil {
			s.logger.WithError(err).Errorf("failed to query open orders for %s", risk.Symbol)
			continue
		}

		// Restore take profit and stop loss orders
		// Add the order to the active order book of the order executor
		m.orderExecutor.ActiveMakerOrders().Add(orders...)
		m.orderExecutor.OrderStore().Add(orders...)

		switch m.Position.Side() {

		case types.SideTypeBuy:
			for _, order := range orders {
				switch order.Type {
				case types.OrderTypeTakeProfitMarket:
					m.TakeProfitOrders.Add(order)
				case types.OrderTypeStopMarket:
					if order.StopPrice.Compare(m.Position.AverageCost) < 0 {
						m.StopLossOrders.Add(order)
					} else {
						m.TakeProfitOrders.Add(order)
					}
				}
			}

		case types.SideTypeSell:
			for _, order := range orders {
				switch order.Type {
				case types.OrderTypeTakeProfitMarket:
					m.TakeProfitOrders.Add(order)
				case types.OrderTypeStopMarket:
					if order.StopPrice.Compare(m.Position.AverageCost) > 0 {
						m.StopLossOrders.Add(order)
					} else {
						m.TakeProfitOrders.Add(order)
					}
				}
			}

		}

		m.logger.Infof("updated position: %+v", m.Position)
		bbgo.Notify("TradingManager %s loaded position", m.Position.Symbol, m.Position)

		bbgo.Notify(m)
	}

	return nil
}

func (s *Strategy) openPositions(ctx context.Context, openPositions []OpenPositionParams) error {
	for _, pos := range openPositions {
		if err := s.OpenPosition(ctx, pos); err != nil {
			return fmt.Errorf("open position error: %w", err)
		}
	}

	return nil
}

// Run implements the strategy interface for strategy execution
func (s *Strategy) Run(ctx context.Context, _ bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	s.session = session

	if riskService, ok := session.Exchange.(types.ExchangeRiskService); ok {
		s.riskService = riskService
	}

	if s.session.Futures && s.riskService != nil {
		if err := s.loadFromPositionRisks(ctx); err != nil {
			return err
		}
	}

	if len(s.OpenPositions) > 0 {
		if len(s.tradingManagers) > 0 {
			s.logger.Warnf("ignoring OpenPositions as trading managers are already initialized")
		} else {
			session.UserDataConnectivity.OnAuth(func() {
				if err := s.openPositions(ctx, s.OpenPositions); err != nil {
					s.logger.WithError(err).Error("failed to open positions")
				}
			})
		}
	}

	bbgo.OnShutdown(ctx, func(ctx context.Context, wg *sync.WaitGroup) {
		defer wg.Done()

		bbgo.Sync(ctx, s)

		defer bbgo.Sync(ctx, s)

		if s.ClosePositionsOnShutdown && len(s.OpenPositions) > 0 {
			s.logger.Infof("closing open positions on shutdown...")

			for _, manager := range s.tradingManagers {
				if !s.HasPosition() {
					s.logger.Warnf("trading manager for symbol %s has no position, skipping close", manager.market.Symbol)
					continue
				}

				if err := manager.ClosePosition(ctx); err != nil {
					s.logger.WithError(err).Errorf("failed to close position for symbol %s", manager.market.Symbol)
				} else {
					s.logger.Infof("closed position for symbol %s", manager.market.Symbol)
				}
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

	if err := m.SetLeverage(ctx, s.Leverage); err != nil {
		return err
	}

	if err := m.OpenPosition(ctx, param); err != nil {
		return fmt.Errorf("open position error: %w", err)
	}

	bbgo.Notify(m)
	return nil
}

func (s *Strategy) HasPosition() bool {
	for _, manager := range s.tradingManagers {
		if manager.Position != nil && manager.Position.Symbol != "" && !manager.Position.GetBase().IsZero() {
			return true
		}
	}
	return false
}
