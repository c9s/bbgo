package supertrend

import (
	"context"
	"fmt"
	"sync"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/types"
)

const ID = "supertrend"

const stateKey = "state-v1"

var log = logrus.WithField("strategy", ID)

func init() {
	// Register the pointer of the strategy struct,
	// so that bbgo knows what struct to be used to unmarshal the configs (YAML or JSON)
	// Note: built-in strategies need to imported manually in the bbgo cmd package.
	bbgo.RegisterStrategy(ID, &Strategy{})
}

type Strategy struct {
	*bbgo.Graceful
	*bbgo.Notifiability
	*bbgo.Persistence

	Environment *bbgo.Environment
	session     *bbgo.ExchangeSession
	Market      types.Market

	// persistence fields
	Position    *types.Position    `json:"position,omitempty" persistence:"position"`
	ProfitStats *types.ProfitStats `json:"profitStats,omitempty" persistence:"profit_stats"`

	// Order and trade
	orderStore     *bbgo.OrderStore
	tradeCollector *bbgo.TradeCollector
	// groupID is the group ID used for the strategy instance for canceling orders
	groupID uint32

	stopC chan struct{}

	// Symbol is the market symbol you want to trade
	Symbol string `json:"symbol"`

	// Interval is how long do you want to update your order price and quantity
	Interval types.Interval `json:"interval"`

	// FastDEMAWindow DEMA window for checking breakout
	FastDEMAWindow int `json:"fastDEMAWindow"`
	// SlowDEMAWindow DEMA window for checking breakout
	SlowDEMAWindow int `json:"slowDEMAWindow"`
	fastDEMA       *indicator.DEMA
	slowDEMA       *indicator.DEMA

	// SuperTrend indicator
	// SuperTrend SuperTrend `json:"superTrend"`
	Supertrend *indicator.Supertrend
	// SupertrendWindow ATR window for calculation of supertrend
	SupertrendWindow int `json:"supertrendWindow"`
	// SupertrendMultiplier ATR multiplier for calculation of supertrend
	SupertrendMultiplier float64 `json:"supertrendMultiplier"`

	// Leverage
	Leverage float64 `json:"leverage"`

	// TakeProfitMultiplier TP according to ATR multiple, 0 to disable this
	TakeProfitMultiplier float64 `json:"takeProfitMultiplier"`

	// StopLossByTriggeringK Set SL price to the low of the triggering Kline
	StopLossByTriggeringK bool `json:"stopLossByTriggeringK"`

	// TPSLBySignal TP/SL by reversed signals
	TPSLBySignal bool `json:"tpslBySignal"`

	currentTakeProfitPrice fixedpoint.Value
	currentStopLossPrice   fixedpoint.Value

	// StrategyController
	bbgo.StrategyController
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) InstanceID() string {
	return fmt.Sprintf("%s:%s", ID, s.Symbol)
}

func (s *Strategy) Validate() error {
	if len(s.Symbol) == 0 {
		return errors.New("symbol is required")
	}

	if len(s.Interval) == 0 {
		return errors.New("interval is required")
	}

	if s.Leverage == 0.0 {
		return errors.New("leverage is required")
	}

	return nil
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.Interval})
}

// Position control

func (s *Strategy) CurrentPosition() *types.Position {
	return s.Position
}

func (s *Strategy) ClosePosition(ctx context.Context, percentage fixedpoint.Value) error {
	base := s.Position.GetBase()
	if base.IsZero() {
		return fmt.Errorf("no opened %s position", s.Position.Symbol)
	}

	// make it negative
	quantity := base.Mul(percentage).Abs()
	side := types.SideTypeBuy
	if base.Sign() > 0 {
		side = types.SideTypeSell
	}

	if quantity.Compare(s.Market.MinQuantity) < 0 {
		return fmt.Errorf("%s order quantity %v is too small, less than %v", s.Symbol, quantity, s.Market.MinQuantity)
	}

	orderForm := s.generateOrderForm(side, quantity, types.SideEffectTypeAutoRepay)

	log.Infof("submit close position order %v", orderForm)
	bbgo.Notify("Submitting %s %s order to close position by %v", s.Symbol, side.String(), percentage)

	createdOrders, err := s.session.Exchange.SubmitOrders(ctx, orderForm)
	if err != nil {
		log.WithError(err).Errorf("can not place %s position close order", s.Symbol)
		bbgo.Notify("can not place %s position close order", s.Symbol)
	}

	s.orderStore.Add(createdOrders...)

	s.tradeCollector.Process()

	_ = s.Persistence.Sync(s)

	return err
}

// setupIndicators initializes indicators
func (s *Strategy) setupIndicators() {
	if s.FastDEMAWindow == 0 {
		s.FastDEMAWindow = 144
	}
	s.fastDEMA = &indicator.DEMA{IntervalWindow: types.IntervalWindow{Interval: s.Interval, Window: s.FastDEMAWindow}}

	if s.SlowDEMAWindow == 0 {
		s.SlowDEMAWindow = 169
	}
	s.slowDEMA = &indicator.DEMA{IntervalWindow: types.IntervalWindow{Interval: s.Interval, Window: s.SlowDEMAWindow}}

	if s.SupertrendWindow == 0 {
		s.SupertrendWindow = 39
	}
	if s.SupertrendMultiplier == 0 {
		s.SupertrendMultiplier = 3
	}
	s.Supertrend = &indicator.Supertrend{IntervalWindow: types.IntervalWindow{Window: s.SupertrendWindow, Interval: s.Interval}, ATRMultiplier: s.SupertrendMultiplier}
	s.Supertrend.AverageTrueRange = &indicator.ATR{IntervalWindow: types.IntervalWindow{Window: s.SupertrendWindow, Interval: s.Interval}}

}

// updateIndicators updates indicators
func (s *Strategy) updateIndicators(kline types.KLine) {
	closePrice := kline.GetClose().Float64()

	// Update indicators
	if kline.Interval == s.fastDEMA.Interval {
		s.fastDEMA.Update(closePrice)
	}
	if kline.Interval == s.slowDEMA.Interval {
		s.slowDEMA.Update(closePrice)
	}
	if kline.Interval == s.Supertrend.Interval {
		s.Supertrend.Update(kline.GetHigh().Float64(), kline.GetLow().Float64(), closePrice)
	}
}

func (s *Strategy) generateOrderForm(side types.SideType, quantity fixedpoint.Value, marginOrderSideEffect types.MarginOrderSideEffectType) types.SubmitOrder {
	orderForm := types.SubmitOrder{
		Symbol:           s.Symbol,
		Market:           s.Market,
		Side:             side,
		Type:             types.OrderTypeMarket,
		Quantity:         quantity,
		MarginSideEffect: marginOrderSideEffect,
	}

	return orderForm
}

// calculateQuantity returns leveraged quantity
func (s *Strategy) calculateQuantity(currentPrice fixedpoint.Value) fixedpoint.Value {
	balance, ok := s.session.GetAccount().Balance(s.Market.QuoteCurrency)
	if !ok {
		log.Errorf("can not update %s balance from exchange", s.Symbol)
		return fixedpoint.Zero
	}

	amountAvailable := balance.Available.Mul(fixedpoint.NewFromFloat(s.Leverage))
	quantity := amountAvailable.Div(currentPrice)

	return quantity
}

func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	s.session = session

	// If position is nil, we need to allocate a new position for calculation
	if s.Position == nil {
		s.Position = types.NewPositionFromMarket(s.Market)
	}
	// Always update the position fields
	s.Position.Strategy = ID
	s.Position.StrategyInstanceID = s.InstanceID()

	s.stopC = make(chan struct{})

	// Profit
	if s.ProfitStats == nil {
		s.ProfitStats = types.NewProfitStats(s.Market)
	}

	s.orderStore = bbgo.NewOrderStore(s.Symbol)
	s.orderStore.BindStream(session.UserDataStream)

	// StrategyController
	s.Status = types.StrategyStatusRunning

	s.OnSuspend(func() {
		_ = s.Persistence.Sync(s)
	})

	s.OnEmergencyStop(func() {
		// Close 100% position
		if err := s.ClosePosition(ctx, fixedpoint.One); err != nil {
			bbgo.Notify("can not close position")
		}
	})

	// Setup indicators
	s.setupIndicators()

	s.currentStopLossPrice = fixedpoint.Zero
	s.currentTakeProfitPrice = fixedpoint.Zero

	session.MarketDataStream.OnKLineClosed(func(kline types.KLine) {
		// StrategyController
		if s.Status != types.StrategyStatusRunning {
			return
		}

		// skip k-lines from other symbols or other intervals
		if kline.Symbol != s.Symbol || kline.Interval != s.Interval {
			return
		}

		// Update indicators
		s.updateIndicators(kline)

		// Get signals
		closePrice := kline.GetClose().Float64()
		openPrice := kline.GetOpen().Float64()
		stSignal := s.Supertrend.GetSignal()
		var demaSignal types.Direction
		if closePrice > s.fastDEMA.Last() && closePrice > s.slowDEMA.Last() && !(openPrice > s.fastDEMA.Last() && openPrice > s.slowDEMA.Last()) {
			demaSignal = types.DirectionUp
		} else if closePrice < s.fastDEMA.Last() && closePrice < s.slowDEMA.Last() && !(openPrice < s.fastDEMA.Last() && openPrice < s.slowDEMA.Last()) {
			demaSignal = types.DirectionDown
		} else {
			demaSignal = types.DirectionNone
		}

		base := s.Position.GetBase()
		baseSign := base.Sign()

		// TP/SL if there's non-dust position
		if !s.Market.IsDustQuantity(base.Abs(), kline.GetClose()) {
			if s.StopLossByTriggeringK && !s.currentStopLossPrice.IsZero() && ((baseSign < 0 && kline.GetClose().Compare(s.currentStopLossPrice) > 0) || (baseSign > 0 && kline.GetClose().Compare(s.currentStopLossPrice) < 0)) {
				// SL by triggering Kline low
				log.Infof("%s SL by triggering Kline low", s.Symbol)
				bbgo.Notify("%s StopLoss by triggering the kline low", s.Symbol)
				if err := s.ClosePosition(ctx, fixedpoint.One); err == nil {
					s.currentStopLossPrice = fixedpoint.Zero
					s.currentTakeProfitPrice = fixedpoint.Zero
				}
			} else if s.TakeProfitMultiplier > 0 && !s.currentTakeProfitPrice.IsZero() && ((baseSign < 0 && kline.GetClose().Compare(s.currentTakeProfitPrice) < 0) || (baseSign > 0 && kline.GetClose().Compare(s.currentTakeProfitPrice) > 0)) {
				// TP by multiple of ATR
				log.Infof("%s TP by multiple of ATR", s.Symbol)
				bbgo.Notify("%s TakeProfit by multiple of ATR", s.Symbol)
				if err := s.ClosePosition(ctx, fixedpoint.One); err == nil {
					s.currentStopLossPrice = fixedpoint.Zero
					s.currentTakeProfitPrice = fixedpoint.Zero
				}
			} else if s.TPSLBySignal {
				// Use signals to TP/SL
				log.Infof("%s TP/SL by reverse of DEMA or Supertrend", s.Symbol)
				bbgo.Notify("%s TP/SL by reverse of DEMA or Supertrend", s.Symbol)
				if (baseSign < 0 && (stSignal == types.DirectionUp || demaSignal == types.DirectionUp)) || (baseSign > 0 && (stSignal == types.DirectionDown || demaSignal == types.DirectionDown)) {
					if err := s.ClosePosition(ctx, fixedpoint.One); err == nil {
						s.currentStopLossPrice = fixedpoint.Zero
						s.currentTakeProfitPrice = fixedpoint.Zero
					}
				}
			}
		}

		// Open position
		var side types.SideType
		if stSignal == types.DirectionUp && demaSignal == types.DirectionUp {
			side = types.SideTypeBuy
			if s.StopLossByTriggeringK {
				s.currentStopLossPrice = kline.GetLow()
			}
			if s.TakeProfitMultiplier > 0 {
				s.currentTakeProfitPrice = kline.GetClose().Add(fixedpoint.NewFromFloat(s.Supertrend.AverageTrueRange.Last() * s.TakeProfitMultiplier))
			}
		} else if stSignal == types.DirectionDown && demaSignal == types.DirectionDown {
			side = types.SideTypeSell
			if s.StopLossByTriggeringK {
				s.currentStopLossPrice = kline.GetHigh()
			}
			if s.TakeProfitMultiplier > 0 {
				s.currentTakeProfitPrice = kline.GetClose().Sub(fixedpoint.NewFromFloat(s.Supertrend.AverageTrueRange.Last() * s.TakeProfitMultiplier))
			}
		}

		// The default value of side is an empty string. Unless side is set by the checks above, the result of the following condition is false
		if side == types.SideTypeSell || side == types.SideTypeBuy {
			log.Infof("open %s position for signal %v", s.Symbol, side)
			bbgo.Notify("open %s position for signal %v", s.Symbol, side)
			// Close opposite position if any
			if !s.Position.IsDust(kline.GetClose()) {
				if (side == types.SideTypeSell && s.Position.IsLong()) || (side == types.SideTypeBuy && s.Position.IsShort()) {
					log.Infof("close existing %s position before open a new position", s.Symbol)
					bbgo.Notify("close existing %s position before open a new position", s.Symbol)
					_ = s.ClosePosition(ctx, fixedpoint.One)
				} else {
					log.Infof("existing %s position has the same direction with the signal", s.Symbol)
					bbgo.Notify("existing %s position has the same direction with the signal", s.Symbol)
					return
				}
			}

			orderForm := s.generateOrderForm(side, s.calculateQuantity(kline.GetClose()), types.SideEffectTypeMarginBuy)
			log.Infof("submit open position order %v", orderForm)
			order, err := orderExecutor.SubmitOrders(ctx, orderForm)
			if err != nil {
				log.WithError(err).Errorf("can not place %s open position order", s.Symbol)
				bbgo.Notify("can not place %s open position order", s.Symbol)
			} else {
				s.orderStore.Add(order...)
			}

			s.tradeCollector.Process()

			_ = s.Persistence.Sync(s)
		}
	})

	s.tradeCollector = bbgo.NewTradeCollector(s.Symbol, s.Position, s.orderStore)

	// Record profits
	s.tradeCollector.OnTrade(func(trade types.Trade, profit, netProfit fixedpoint.Value) {
		bbgo.Notify(trade)
		s.ProfitStats.AddTrade(trade)

		if profit.Compare(fixedpoint.Zero) == 0 {
			s.Environment.RecordPosition(s.Position, trade, nil)
		} else {
			log.Infof("%s generated profit: %v", s.Symbol, profit)
			p := s.Position.NewProfit(trade, profit, netProfit)
			p.Strategy = ID
			p.StrategyInstanceID = s.InstanceID()
			bbgo.Notify(&p)

			s.ProfitStats.AddProfit(p)
			bbgo.Notify(&s.ProfitStats)

			s.Environment.RecordPosition(s.Position, trade, &p)
		}
	})

	s.tradeCollector.BindStream(session.UserDataStream)

	// Graceful shutdown
	s.Graceful.OnShutdown(func(ctx context.Context, wg *sync.WaitGroup) {
		defer wg.Done()
		close(s.stopC)

		s.tradeCollector.Process()
	})

	return nil
}
