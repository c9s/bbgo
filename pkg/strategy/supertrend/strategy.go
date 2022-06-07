package supertrend

import (
	"context"
	"fmt"
	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"math"
	"sync"
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

type SuperTrend struct {
	// AverageTrueRangeWindow ATR window for calculation of supertrend
	AverageTrueRangeWindow int `json:"averageTrueRangeWindow"`
	// AverageTrueRangeMultiplier ATR multiplier for calculation of supertrend
	AverageTrueRangeMultiplier float64 `json:"averageTrueRangeMultiplier"`

	averageTrueRange *indicator.ATR

	closePrice         float64
	lastClosePrice     float64
	uptrendPrice       float64
	lastUptrendPrice   float64
	downtrendPrice     float64
	lastDowntrendPrice float64

	trend       types.Direction
	lastTrend   types.Direction
	tradeSignal types.Direction
}

// update SuperTrend indicator
func (st *SuperTrend) update(kline types.KLine) {
	highPrice := kline.GetHigh().Float64()
	lowPrice := kline.GetLow().Float64()
	closePrice := kline.GetClose().Float64()

	// Update ATR
	st.averageTrueRange.Update(highPrice, lowPrice, closePrice)

	// Update last prices
	st.lastUptrendPrice = st.uptrendPrice
	st.lastDowntrendPrice = st.downtrendPrice
	st.lastClosePrice = st.closePrice
	st.lastTrend = st.trend

	st.closePrice = closePrice

	src := (highPrice + lowPrice) / 2

	// Update uptrend
	st.uptrendPrice = src - st.averageTrueRange.Last()*st.AverageTrueRangeMultiplier
	if st.lastClosePrice > st.lastUptrendPrice {
		st.uptrendPrice = math.Max(st.uptrendPrice, st.lastUptrendPrice)
	}

	// Update downtrend
	st.downtrendPrice = src + st.averageTrueRange.Last()*st.AverageTrueRangeMultiplier
	if st.lastClosePrice < st.lastDowntrendPrice {
		st.downtrendPrice = math.Min(st.downtrendPrice, st.lastDowntrendPrice)
	}

	// Update trend
	if st.lastTrend == types.DirectionUp && st.closePrice < st.lastUptrendPrice {
		st.trend = types.DirectionDown
	} else if st.lastTrend == types.DirectionDown && st.closePrice > st.lastDowntrendPrice {
		st.trend = types.DirectionUp
	} else {
		st.trend = st.lastTrend
	}

	// Update signal
	if st.trend == types.DirectionUp && st.lastTrend == types.DirectionDown {
		st.tradeSignal = types.DirectionUp
	} else if st.trend == types.DirectionDown && st.lastTrend == types.DirectionUp {
		st.tradeSignal = types.DirectionDown
	} else {
		st.tradeSignal = types.DirectionNone
	}
}

// getSignal returns SuperTrend signal
func (st *SuperTrend) getSignal() types.Direction {
	return st.tradeSignal
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
	SuperTrend SuperTrend `json:"superTrend"`

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
		return fmt.Errorf("order quantity %v is too small, less than %v", quantity, s.Market.MinQuantity)
	}

	orderForm := s.generateOrderForm(side, quantity, types.SideEffectTypeAutoRepay)

	s.Notify("Submitting %s %s order to close position by %v", s.Symbol, side.String(), percentage, orderForm)

	createdOrders, err := s.session.Exchange.SubmitOrders(ctx, orderForm)
	if err != nil {
		log.WithError(err).Errorf("can not place position close order")
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

	if s.SuperTrend.AverageTrueRangeWindow == 0 {
		s.SuperTrend.AverageTrueRangeWindow = 39
	}
	s.SuperTrend.averageTrueRange = &indicator.ATR{IntervalWindow: types.IntervalWindow{Window: s.SuperTrend.AverageTrueRangeWindow, Interval: s.Interval}}
	s.SuperTrend.trend = types.DirectionUp
	if s.SuperTrend.AverageTrueRangeMultiplier == 0 {
		s.SuperTrend.AverageTrueRangeMultiplier = 3
	}
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
	if kline.Interval == s.SuperTrend.averageTrueRange.Interval {
		s.SuperTrend.update(kline)
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
		log.Error("can not update balance from exchange")
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
			s.Notify("can not close position")
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
		stSignal := s.SuperTrend.getSignal()
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
				// SL by triggered Kline low
				if err := s.ClosePosition(ctx, fixedpoint.One); err != nil {
					s.Notify("can not place SL order")
				} else {
					s.currentStopLossPrice = fixedpoint.Zero
					s.currentTakeProfitPrice = fixedpoint.Zero
				}
			} else if s.TakeProfitMultiplier > 0 && !s.currentTakeProfitPrice.IsZero() && ((baseSign < 0 && kline.GetClose().Compare(s.currentTakeProfitPrice) < 0) || (baseSign > 0 && kline.GetClose().Compare(s.currentTakeProfitPrice) > 0)) {
				// TP by multiple of ATR
				if err := s.ClosePosition(ctx, fixedpoint.One); err != nil {
					s.Notify("can not place TP order")
				} else {
					s.currentStopLossPrice = fixedpoint.Zero
					s.currentTakeProfitPrice = fixedpoint.Zero
				}
			} else if s.TPSLBySignal {
				// Use signals to TP/SL
				if (baseSign < 0 && (stSignal == types.DirectionUp || demaSignal == types.DirectionUp)) || (baseSign > 0 && (stSignal == types.DirectionDown || demaSignal == types.DirectionDown)) {
					if err := s.ClosePosition(ctx, fixedpoint.One); err != nil {
						s.Notify("can not place TP/SL order")
					} else {
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
				s.currentTakeProfitPrice = kline.GetClose().Add(fixedpoint.NewFromFloat(s.SuperTrend.averageTrueRange.Last() * s.TakeProfitMultiplier))
			}
		} else if stSignal == types.DirectionDown && demaSignal == types.DirectionDown {
			side = types.SideTypeSell
			if s.StopLossByTriggeringK {
				s.currentStopLossPrice = kline.GetHigh()
			}
			if s.TakeProfitMultiplier > 0 {
				s.currentTakeProfitPrice = kline.GetClose().Sub(fixedpoint.NewFromFloat(s.SuperTrend.averageTrueRange.Last() * s.TakeProfitMultiplier))
			}
		}

		if side == types.SideTypeSell || side == types.SideTypeBuy {
			// Close opposite position if any
			if !s.Market.IsDustQuantity(base.Abs(), kline.GetClose()) && ((side == types.SideTypeSell && baseSign > 0) || (side == types.SideTypeBuy && baseSign < 0)) {
				if err := s.ClosePosition(ctx, fixedpoint.One); err != nil {
					s.Notify("can not place close position order")
				}
			}

			orderForm := s.generateOrderForm(side, s.calculateQuantity(kline.GetClose()), types.SideEffectTypeMarginBuy)
			log.Infof("submit open position order %v", orderForm)
			order, err := orderExecutor.SubmitOrders(ctx, orderForm)
			if err != nil {
				log.WithError(err).Errorf("can not place open position order")
				s.Notify("can not place open position order")
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
		s.Notifiability.Notify(trade)
		s.ProfitStats.AddTrade(trade)

		if profit.Compare(fixedpoint.Zero) == 0 {
			s.Environment.RecordPosition(s.Position, trade, nil)
		} else {
			log.Infof("%s generated profit: %v", s.Symbol, profit)
			p := s.Position.NewProfit(trade, profit, netProfit)
			p.Strategy = ID
			p.StrategyInstanceID = s.InstanceID()
			s.Notify(&p)

			s.ProfitStats.AddProfit(p)
			s.Notify(&s.ProfitStats)

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
