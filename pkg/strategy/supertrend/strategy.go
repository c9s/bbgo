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
)

// TODO:
// 1. Position control
// 2. Strategy control

const ID = "supertrend"

const stateKey = "state-v1"

var NotionalModifier = fixedpoint.NewFromFloat(1.0001)

var zeroiw = types.IntervalWindow{}

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

	AverageTrueRange *indicator.ATR

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

// Update SuperTrend indicator
func (st *SuperTrend) Update(kline types.KLine) {
	highPrice := kline.GetHigh().Float64()
	lowPrice := kline.GetLow().Float64()
	closePrice := kline.GetClose().Float64()

	// Update ATR
	st.AverageTrueRange.Update(highPrice, lowPrice, closePrice)

	// Update last prices
	st.lastUptrendPrice = st.uptrendPrice
	st.lastDowntrendPrice = st.downtrendPrice
	st.lastClosePrice = st.closePrice
	st.lastTrend = st.trend

	st.closePrice = closePrice

	src := (highPrice + lowPrice) / 2

	// Update uptrend
	st.uptrendPrice = src - st.AverageTrueRange.Last()*st.AverageTrueRangeMultiplier
	if st.lastClosePrice > st.lastUptrendPrice {
		st.uptrendPrice = math.Max(st.uptrendPrice, st.lastUptrendPrice)
	}

	// Update downtrend
	st.downtrendPrice = src + st.AverageTrueRange.Last()*st.AverageTrueRangeMultiplier
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

// GetSignal returns SuperTrend signal
func (st *SuperTrend) GetSignal() types.Direction {
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

	// Symbol is the market symbol you want to trade
	Symbol string `json:"symbol"`

	// Interval is how long do you want to update your order price and quantity
	Interval types.Interval `json:"interval"`

	// FastDEMA DEMA window for checking breakout
	FastDEMAWindow int `json:"fastDEMAWindow"`
	// SlowDEMA DEMA window for checking breakout
	SlowDEMAWindow int `json:"slowDEMAWindow"`
	FastDEMA       *indicator.DEMA
	SlowDEMA       *indicator.DEMA

	// SuperTrend indicator
	SuperTrend SuperTrend `json:"superTrend"`

	bbgo.QuantityOrAmount

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

	// TODO: Validate DEMA window and ATR window
	return nil
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.Interval})
}

// SetupIndicators initializes indicators
func (s *Strategy) SetupIndicators() {
	if s.FastDEMAWindow == 0 {
		s.FastDEMAWindow = 144
	}
	s.FastDEMA = &indicator.DEMA{IntervalWindow: types.IntervalWindow{Interval: s.Interval, Window: s.FastDEMAWindow}}

	if s.SlowDEMAWindow == 0 {
		s.SlowDEMAWindow = 169
	}
	s.SlowDEMA = &indicator.DEMA{IntervalWindow: types.IntervalWindow{Interval: s.Interval, Window: s.SlowDEMAWindow}}

	if s.SuperTrend.AverageTrueRangeWindow == 0 {
		s.SuperTrend.AverageTrueRangeWindow = 43
	}
	s.SuperTrend.AverageTrueRange = &indicator.ATR{IntervalWindow: types.IntervalWindow{Window: s.SuperTrend.AverageTrueRangeWindow, Interval: s.Interval}}
	s.SuperTrend.trend = types.DirectionUp
	if s.SuperTrend.AverageTrueRangeMultiplier == 0 {
		s.SuperTrend.AverageTrueRangeMultiplier = 4
	}
}

func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	s.session = session
	s.Market, _ = session.Market(s.Symbol)

	// If position is nil, we need to allocate a new position for calculation
	if s.Position == nil {
		s.Position = types.NewPositionFromMarket(s.Market)
	}
	// Always update the position fields
	s.Position.Strategy = ID
	s.Position.StrategyInstanceID = s.InstanceID()

	if s.ProfitStats == nil {
		s.ProfitStats = types.NewProfitStats(s.Market)
	}

	s.orderStore = bbgo.NewOrderStore(s.Symbol)
	s.orderStore.BindStream(session.UserDataStream)

	// StrategyController
	s.Status = types.StrategyStatusRunning

	// Setup indicators
	s.SetupIndicators()

	session.MarketDataStream.OnKLineClosed(func(kline types.KLine) {
		// skip k-lines from other symbols
		if kline.Symbol != s.Symbol {
			return
		}

		closePrice := kline.GetClose().Float64()
		openPrice := kline.GetOpen().Float64()

		// Update indicators
		if kline.Interval == s.FastDEMA.Interval {
			s.FastDEMA.Update(closePrice)
		}
		if kline.Interval == s.SlowDEMA.Interval {
			s.SlowDEMA.Update(closePrice)
		}
		if kline.Interval == s.SuperTrend.AverageTrueRange.Interval {
			s.SuperTrend.Update(kline)
		}

		if kline.Symbol != s.Symbol || kline.Interval != s.Interval {
			return
		}

		// Get signals
		stSignal := s.SuperTrend.GetSignal()
		var demaSignal types.Direction
		if closePrice > s.FastDEMA.Last() && closePrice > s.SlowDEMA.Last() && !(openPrice > s.FastDEMA.Last() && openPrice > s.SlowDEMA.Last()) {
			demaSignal = types.DirectionUp
		} else if closePrice < s.FastDEMA.Last() && closePrice < s.SlowDEMA.Last() && !(openPrice < s.FastDEMA.Last() && openPrice < s.SlowDEMA.Last()) {
			demaSignal = types.DirectionDown
		} else {
			demaSignal = types.DirectionNone
		}

		// TP/SL
		base := s.Position.GetBase()
		quantity := base.Abs()
		if quantity.Compare(s.Market.MinQuantity) > 0 && quantity.Mul(kline.GetClose()).Compare(s.Market.MinNotional) > 0 {
			if base.Sign() < 0 && (stSignal == types.DirectionUp || demaSignal == types.DirectionUp) {
				orderForm := types.SubmitOrder{
					Symbol:   s.Symbol,
					Market:   s.Market,
					Side:     types.SideTypeBuy,
					Type:     types.OrderTypeMarket,
					Quantity: quantity,
				}
				log.Infof("submit TP/SL order %v", orderForm)
				createdOrder, err := s.session.Exchange.SubmitOrders(ctx, orderForm)
				if err != nil {
					log.WithError(err).Errorf("can not place TP/SL order")
				}
				s.orderStore.Add(createdOrder...)
			} else if base.Sign() > 0 && (stSignal == types.DirectionDown || demaSignal == types.DirectionDown) {
				orderForm := types.SubmitOrder{
					Symbol:   s.Symbol,
					Market:   s.Market,
					Side:     types.SideTypeSell,
					Type:     types.OrderTypeMarket,
					Quantity: quantity,
				}
				log.Infof("submit TP/SL order %v", orderForm)
				createdOrder, err := s.session.Exchange.SubmitOrders(ctx, orderForm)
				if err != nil {
					log.WithError(err).Errorf("can not place TP/SL order")
				}
				s.orderStore.Add(createdOrder...)
			}
		}

		// Place order
		var side types.SideType
		if stSignal == types.DirectionUp && demaSignal == types.DirectionUp {
			side = types.SideTypeBuy
		} else if stSignal == types.DirectionDown && demaSignal == types.DirectionDown {
			side = types.SideTypeSell
		}

		balance, _ := s.session.GetAccount().Balance(s.Market.QuoteCurrency)
		s.Amount = balance.Available
		if side == types.SideTypeSell || side == types.SideTypeBuy {
			orderForm := types.SubmitOrder{
				Symbol:   s.Symbol,
				Market:   s.Market,
				Side:     side,
				Type:     types.OrderTypeMarket,
				Quantity: s.CalculateQuantity(fixedpoint.NewFromFloat(closePrice)),
			}

			createdOrder, err := s.session.Exchange.SubmitOrders(ctx, orderForm)
			if err != nil {
				log.WithError(err).Errorf("can not place order")
			}
			s.orderStore.Add(createdOrder...)
		}

		// check if there is a canceled order had partially filled.
		s.tradeCollector.Process()
	})

	s.tradeCollector = bbgo.NewTradeCollector(s.Symbol, s.Position, s.orderStore)

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

	return nil
}
