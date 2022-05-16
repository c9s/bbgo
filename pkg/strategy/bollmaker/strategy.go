package bollmaker

import (
	"context"
	"fmt"
	"math"
	"sync"

	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/util"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

// TODO:
// 1) add option for placing orders only when in neutral band
// 2) add option for only placing buy orders when price is below the SMA line

const ID = "bollmaker"

const stateKey = "state-v1"

var defaultFeeRate = fixedpoint.NewFromFloat(0.001)
var notionModifier = fixedpoint.NewFromFloat(1.1)
var two = fixedpoint.NewFromInt(2)

var log = logrus.WithField("strategy", ID)

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

// Deprecated: State is deprecated, please use the persistence tag
type State struct {
	// Deprecated: Position is deprecated, please define the Position field in the strategy struct directly.
	Position *types.Position `json:"position,omitempty"`

	// Deprecated: ProfitStats is deprecated, please define the ProfitStats field in the strategy struct directly.
	ProfitStats types.ProfitStats `json:"profitStats,omitempty"`
}

type BollingerSetting struct {
	types.IntervalWindow
	BandWidth float64 `json:"bandWidth"`
}

type Strategy struct {
	*bbgo.Graceful
	*bbgo.Notifiability
	*bbgo.Persistence

	Environment          *bbgo.Environment
	StandardIndicatorSet *bbgo.StandardIndicatorSet
	Market               types.Market

	// Symbol is the market symbol you want to trade
	Symbol string `json:"symbol"`

	// Interval is how long do you want to update your order price and quantity
	Interval types.Interval `json:"interval"`

	bbgo.QuantityOrAmount

	// Spread is the price spread from the middle price.
	// For ask orders, the ask price is ((bestAsk + bestBid) / 2 * (1.0 + spread))
	// For bid orders, the bid price is ((bestAsk + bestBid) / 2 * (1.0 - spread))
	// Spread can be set by percentage or floating number. e.g., 0.1% or 0.001
	Spread fixedpoint.Value `json:"spread"`

	// BidSpread overrides the spread setting, this spread will be used for the buy order
	BidSpread fixedpoint.Value `json:"bidSpread,omitempty"`

	// AskSpread overrides the spread setting, this spread will be used for the sell order
	AskSpread fixedpoint.Value `json:"askSpread,omitempty"`

	// DynamicSpreadWindow enables the automatic adjustment to bid and ask spread.
	// When DynamicSpreadWindow is set and is larger than 0, the spreads are calculated based on the SMA of amplitude of
	// [DynamicSpreadWindow] K-lines
	DynamicSpreadWindow int              `json:"dynamicSpreadWindow,omitempty"`
	MinAskSpread        fixedpoint.Value `json:"minAskSpread"`
	MinBidSpread        fixedpoint.Value `json:"minBidSpread"`
	MaxAskSpread        fixedpoint.Value `json:"maxAskSpread"`
	MaxBidSpread        fixedpoint.Value `json:"maxBidSpread"`

	DynamicAskSpread *indicator.SMA
	DynamicBidSpread *indicator.SMA

	// MinProfitSpread is the minimal order price spread from the current average cost.
	// For long position, you will only place sell order above the price (= average cost * (1 + minProfitSpread))
	// For short position, you will only place buy order below the price (= average cost * (1 - minProfitSpread))
	MinProfitSpread fixedpoint.Value `json:"minProfitSpread"`

	// UseTickerPrice use the ticker api to get the mid price instead of the closed kline price.
	// The back-test engine is kline-based, so the ticker price api is not supported.
	// Turn this on if you want to do real trading.
	UseTickerPrice bool `json:"useTickerPrice"`

	// MaxExposurePosition is the maximum position you can hold
	// +10 means you can hold 10 ETH long position by maximum
	// -10 means you can hold -10 ETH short position by maximum
	MaxExposurePosition fixedpoint.Value `json:"maxExposurePosition"`

	// DynamicExposurePositionScale is used to define the exposure position range with the given percentage
	// when DynamicExposurePositionScale is set,
	// your MaxExposurePosition will be calculated dynamically according to the bollinger band you set.
	DynamicExposurePositionScale *bbgo.PercentageScale `json:"dynamicExposurePositionScale"`

	// Long means your position will be long position
	// Currently not used yet
	Long *bool `json:"long,omitempty"`

	// Short means your position will be long position
	// Currently not used yet
	Short *bool `json:"short,omitempty"`

	// DisableShort means you can don't want short position during the market making
	// Set to true if you want to hold more spot during market making.
	DisableShort bool `json:"disableShort"`

	// BuyBelowNeutralSMA if true, the market maker will only place buy order when the current price is below the neutral band SMA.
	BuyBelowNeutralSMA bool `json:"buyBelowNeutralSMA"`

	// NeutralBollinger is the smaller range of the bollinger band
	// If price is in this band, it usually means the price is oscillating.
	// If price goes out of this band, we tend to not place sell orders or buy orders
	NeutralBollinger *BollingerSetting `json:"neutralBollinger"`

	// DefaultBollinger is the wide range of the bollinger band
	// for controlling your exposure position
	DefaultBollinger *BollingerSetting `json:"defaultBollinger"`

	// DowntrendSkew is the order quantity skew for normal downtrend band.
	// The price is still in the default bollinger band.
	// greater than 1.0 means when placing buy order, place sell order with less quantity
	// less than 1.0 means when placing sell order, place buy order with less quantity
	DowntrendSkew fixedpoint.Value `json:"downtrendSkew"`

	// UptrendSkew is the order quantity skew for normal uptrend band.
	// The price is still in the default bollinger band.
	// greater than 1.0 means when placing buy order, place sell order with less quantity
	// less than 1.0 means when placing sell order, place buy order with less quantity
	UptrendSkew fixedpoint.Value `json:"uptrendSkew"`

	// TradeInBand
	// When this is on, places orders only when the current price is in the bollinger band.
	TradeInBand bool `json:"tradeInBand"`

	// ShadowProtection is used to avoid placing bid order when price goes down strongly (without shadow)
	ShadowProtection      bool             `json:"shadowProtection"`
	ShadowProtectionRatio fixedpoint.Value `json:"shadowProtectionRatio"`

	bbgo.SmartStops

	session *bbgo.ExchangeSession
	book    *types.StreamOrderBook

	state *State

	// persistence fields
	Position    *types.Position    `json:"position,omitempty" persistence:"position"`
	ProfitStats *types.ProfitStats `json:"profitStats,omitempty" persistence:"profit_stats"`

	activeMakerOrders *bbgo.LocalActiveOrderBook
	orderStore        *bbgo.OrderStore
	tradeCollector    *bbgo.TradeCollector

	groupID uint32

	stopC chan struct{}

	// defaultBoll is the BOLLINGER indicator we used for predicting the price.
	defaultBoll *indicator.BOLL

	// neutralBoll is the neutral price section
	neutralBoll *indicator.BOLL

	// StrategyController
	bbgo.StrategyController
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) InstanceID() string {
	return fmt.Sprintf("%s:%s", ID, s.Symbol)
}

func (s *Strategy) Initialize() error {
	return s.SmartStops.InitializeStopControllers(s.Symbol)
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{
		Interval: string(s.Interval),
	})

	if s.DefaultBollinger != nil && s.DefaultBollinger.Interval != "" {
		session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{
			Interval: string(s.DefaultBollinger.Interval),
		})
	}

	if s.NeutralBollinger != nil && s.NeutralBollinger.Interval != "" {
		session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{
			Interval: string(s.NeutralBollinger.Interval),
		})
	}

	s.SmartStops.Subscribe(session)
}

func (s *Strategy) Validate() error {
	if len(s.Symbol) == 0 {
		return errors.New("symbol is required")
	}

	return nil
}

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

	submitOrder := types.SubmitOrder{
		Symbol:   s.Symbol,
		Side:     side,
		Type:     types.OrderTypeMarket,
		Quantity: quantity,
		Market:   s.Market,
	}

	s.Notify("Submitting %s %s order to close position by %v", s.Symbol, side.String(), percentage, submitOrder)

	createdOrders, err := s.session.Exchange.SubmitOrders(ctx, submitOrder)
	if err != nil {
		log.WithError(err).Errorf("can not place position close order")
	}

	s.orderStore.Add(createdOrders...)
	s.activeMakerOrders.Add(createdOrders...)
	return err
}

// Deprecated: LoadState method is migrated to the persistence struct tag.
func (s *Strategy) LoadState() error {
	var state State

	// load position
	if err := s.Persistence.Load(&state, ID, s.Symbol, stateKey); err == nil {
		s.state = &state
	}

	return nil
}

func (s *Strategy) getCurrentAllowedExposurePosition(bandPercentage float64) (fixedpoint.Value, error) {
	if s.DynamicExposurePositionScale != nil {
		v, err := s.DynamicExposurePositionScale.Scale(bandPercentage)
		if err != nil {
			return fixedpoint.Zero, err
		}
		return fixedpoint.NewFromFloat(v), nil
	}

	return s.MaxExposurePosition, nil
}

func (s *Strategy) placeOrders(ctx context.Context, orderExecutor bbgo.OrderExecutor, midPrice fixedpoint.Value, kline *types.KLine) {
	bidSpread := s.Spread
	if s.BidSpread.Sign() > 0 {
		bidSpread = s.BidSpread
	}

	askSpread := s.Spread
	if s.AskSpread.Sign() > 0 {
		askSpread = s.AskSpread
	}

	askPrice := midPrice.Mul(fixedpoint.One.Add(askSpread))
	bidPrice := midPrice.Mul(fixedpoint.One.Sub(bidSpread))
	base := s.Position.GetBase()
	balances := s.session.GetAccount().Balances()

	log.Infof("mid price:%v spread: %s ask:%v bid: %v position: %s",
		midPrice,
		s.Spread.Percentage(),
		askPrice,
		bidPrice,
		s.Position,
	)

	sellQuantity := s.QuantityOrAmount.CalculateQuantity(askPrice)
	buyQuantity := s.QuantityOrAmount.CalculateQuantity(bidPrice)

	sellOrder := types.SubmitOrder{
		Symbol:   s.Symbol,
		Side:     types.SideTypeSell,
		Type:     types.OrderTypeLimitMaker,
		Quantity: sellQuantity,
		Price:    askPrice,
		Market:   s.Market,
		GroupID:  s.groupID,
	}
	buyOrder := types.SubmitOrder{
		Symbol:   s.Symbol,
		Side:     types.SideTypeBuy,
		Type:     types.OrderTypeLimitMaker,
		Quantity: buyQuantity,
		Price:    bidPrice,
		Market:   s.Market,
		GroupID:  s.groupID,
	}

	var submitOrders []types.SubmitOrder

	baseBalance, hasBaseBalance := balances[s.Market.BaseCurrency]
	quoteBalance, hasQuoteBalance := balances[s.Market.QuoteCurrency]

	downBand := s.defaultBoll.LastDownBand()
	upBand := s.defaultBoll.LastUpBand()
	sma := s.defaultBoll.LastSMA()
	log.Infof("bollinger band: up %f sma %f down %f", upBand, sma, downBand)

	bandPercentage := calculateBandPercentage(upBand, downBand, sma, midPrice.Float64())
	log.Infof("mid price band percentage: %v", bandPercentage)

	maxExposurePosition, err := s.getCurrentAllowedExposurePosition(bandPercentage)
	if err != nil {
		log.WithError(err).Errorf("can not calculate CurrentAllowedExposurePosition")
		return
	}

	log.Infof("calculated max exposure position: %v", maxExposurePosition)

	canSell := true
	canBuy := true

	if maxExposurePosition.Sign() > 0 && base.Compare(maxExposurePosition) > 0 {
		canBuy = false
	}

	if maxExposurePosition.Sign() > 0 {
		if s.Long != nil && *s.Long && base.Sign() < 0 {
			canSell = false
		} else if base.Compare(maxExposurePosition.Neg()) < 0 {
			canSell = false
		}
	}

	if s.ShadowProtection && kline != nil {
		switch kline.Direction() {
		case types.DirectionDown:
			shadowHeight := kline.GetLowerShadowHeight()
			shadowRatio := kline.GetLowerShadowRatio()
			if shadowHeight.IsZero() && shadowRatio.Compare(s.ShadowProtectionRatio) < 0 {
				log.Infof("%s shadow protection enabled, lower shadow ratio %v < %v", s.Symbol, shadowRatio, s.ShadowProtectionRatio)
				canBuy = false
			}
		case types.DirectionUp:
			shadowHeight := kline.GetUpperShadowHeight()
			shadowRatio := kline.GetUpperShadowRatio()
			if shadowHeight.IsZero() || shadowRatio.Compare(s.ShadowProtectionRatio) < 0 {
				log.Infof("%s shadow protection enabled, upper shadow ratio %v < %v", s.Symbol, shadowRatio, s.ShadowProtectionRatio)
				canSell = false
			}
		}
	}

	// Apply quantity skew
	// CASE #1:
	// WHEN: price is in the neutral bollginer band (window 1) == neutral
	// THEN: we don't apply skew
	// CASE #2:
	// WHEN: price is in the upper band (window 2 > price > window 1) == upTrend
	// THEN: we apply upTrend skew
	// CASE #3:
	// WHEN: price is in the lower band (window 2 < price < window 1) == downTrend
	// THEN: we apply downTrend skew
	// CASE #4:
	// WHEN: price breaks the lower band (price < window 2) == strongDownTrend
	// THEN: we apply strongDownTrend skew
	// CASE #5:
	// WHEN: price breaks the upper band (price > window 2) == strongUpTrend
	// THEN: we apply strongUpTrend skew
	if s.TradeInBand {
		if !inBetween(midPrice.Float64(), s.neutralBoll.LastDownBand(), s.neutralBoll.LastUpBand()) {
			log.Infof("tradeInBand is set, skip placing orders when the price is outside of the band")
			return
		}
	}

	trend := s.detectPriceTrend(s.neutralBoll, midPrice.Float64())
	switch trend {
	case NeutralTrend:
		// do nothing

	case UpTrend:
		skew := s.UptrendSkew
		buyOrder.Quantity = fixedpoint.Max(s.Market.MinQuantity, sellOrder.Quantity.Mul(skew))

	case DownTrend:
		skew := s.DowntrendSkew
		ratio := fixedpoint.One.Div(skew)
		sellOrder.Quantity = fixedpoint.Max(s.Market.MinQuantity, buyOrder.Quantity.Mul(ratio))

	}

	if !hasQuoteBalance || buyOrder.Quantity.Mul(buyOrder.Price).Compare(quoteBalance.Available) > 0 {
		canBuy = false
	}

	if !hasBaseBalance || sellOrder.Quantity.Compare(baseBalance.Available) > 0 {
		canSell = false
	}

	if midPrice.Compare(s.Position.AverageCost.Mul(fixedpoint.One.Add(s.MinProfitSpread))) < 0 {
		canSell = false
	}

	if s.Long != nil && *s.Long && base.Sub(sellOrder.Quantity).Sign() < 0 {
		canSell = false
	}

	if s.BuyBelowNeutralSMA && midPrice.Float64() > s.neutralBoll.LastSMA() {
		canBuy = false
	}

	if canSell {
		submitOrders = append(submitOrders, sellOrder)
	}
	if canBuy {
		submitOrders = append(submitOrders, buyOrder)
	}

	// condition for lower the average cost
	/*
		if midPrice < s.Position.AverageCost.MulFloat64(1.0-s.MinProfitSpread.Float64()) && canBuy {
			submitOrders = append(submitOrders, buyOrder)
		}
	*/

	if len(submitOrders) == 0 {
		return
	}

	for i := range submitOrders {
		submitOrders[i] = s.adjustOrderQuantity(submitOrders[i])
	}

	createdOrders, err := orderExecutor.SubmitOrders(ctx, submitOrders...)
	if err != nil {
		log.WithError(err).Errorf("can not place ping pong orders")
	}
	s.orderStore.Add(createdOrders...)
	s.activeMakerOrders.Add(createdOrders...)
}

type PriceTrend string

const (
	NeutralTrend PriceTrend = "neutral"
	UpTrend      PriceTrend = "upTrend"
	DownTrend    PriceTrend = "downTrend"
	UnknownTrend PriceTrend = "unknown"
)

func (s *Strategy) detectPriceTrend(inc *indicator.BOLL, price float64) PriceTrend {
	if inBetween(price, inc.LastDownBand(), inc.LastUpBand()) {
		return NeutralTrend
	}

	if price < inc.LastDownBand() {
		return DownTrend
	}

	if price > inc.LastUpBand() {
		return UpTrend
	}

	return UnknownTrend
}

func (s *Strategy) adjustOrderQuantity(submitOrder types.SubmitOrder) types.SubmitOrder {
	if submitOrder.Quantity.Mul(submitOrder.Price).Compare(s.Market.MinNotional) < 0 {
		submitOrder.Quantity = bbgo.AdjustFloatQuantityByMinAmount(submitOrder.Quantity, submitOrder.Price, s.Market.MinNotional.Mul(notionModifier))
	}

	if submitOrder.Quantity.Compare(s.Market.MinQuantity) < 0 {
		submitOrder.Quantity = fixedpoint.Max(submitOrder.Quantity, s.Market.MinQuantity)
	}

	return submitOrder
}

func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	// StrategyController
	s.Status = types.StrategyStatusRunning

	// Setup dynamic spread
	if s.DynamicSpreadWindow > 0 {
		s.DynamicBidSpread = &indicator.SMA{IntervalWindow: types.IntervalWindow{s.Interval, s.DynamicSpreadWindow}}
		s.DynamicAskSpread = &indicator.SMA{IntervalWindow: types.IntervalWindow{s.Interval, s.DynamicSpreadWindow}}
	}

	s.OnSuspend(func() {
		s.Status = types.StrategyStatusStopped

		// Cancel all order
		if err := s.activeMakerOrders.GracefulCancel(ctx, s.session.Exchange); err != nil {
			log.WithError(err).Errorf("graceful cancel order error")
			s.Notify("graceful cancel order error")
		} else {
			s.Notify("All orders are cancelled.")
		}

		s.tradeCollector.Process()

		_ = s.Persistence.Sync(s)
	})

	s.OnEmergencyStop(func() {
		// Close 100% position
		percentage := fixedpoint.NewFromFloat(1.0)
		_ = s.ClosePosition(ctx, percentage)
	})

	if s.DisableShort {
		s.Long = &[]bool{true}[0]
	}

	if s.MinProfitSpread.IsZero() {
		s.MinProfitSpread = fixedpoint.NewFromFloat(0.001)
	}

	if s.UptrendSkew.IsZero() {
		s.UptrendSkew = fixedpoint.NewFromFloat(1.0 / 1.2)
	}

	if s.DowntrendSkew.IsZero() {
		s.DowntrendSkew = fixedpoint.NewFromFloat(1.2)
	}

	if s.ShadowProtectionRatio.IsZero() {
		s.ShadowProtectionRatio = fixedpoint.NewFromFloat(0.01)
	}

	// initial required information
	s.session = session
	s.neutralBoll = s.StandardIndicatorSet.BOLL(s.NeutralBollinger.IntervalWindow, s.NeutralBollinger.BandWidth)
	s.defaultBoll = s.StandardIndicatorSet.BOLL(s.DefaultBollinger.IntervalWindow, s.DefaultBollinger.BandWidth)

	// calculate group id for orders
	instanceID := s.InstanceID()
	s.groupID = util.FNV32(instanceID)

	// restore state
	if err := s.LoadState(); err != nil {
		return err
	}

	// If position is nil, we need to allocate a new position for calculation
	if s.Position == nil {
		// fallback to the legacy position struct in the state
		if s.state != nil && s.state.Position != nil {
			s.Position = s.state.Position
		} else {
			s.Position = types.NewPositionFromMarket(s.Market)
		}
	}

	if s.ProfitStats == nil {
		if s.state != nil {
			// copy profit stats
			p2 := s.state.ProfitStats
			s.ProfitStats = &p2
		} else {
			s.ProfitStats = types.NewProfitStats(s.Market)
		}
	}

	// Always update the position fields
	s.Position.Strategy = ID
	s.Position.StrategyInstanceID = instanceID

	s.stopC = make(chan struct{})

	s.activeMakerOrders = bbgo.NewLocalActiveOrderBook(s.Symbol)
	s.activeMakerOrders.BindStream(session.UserDataStream)

	s.orderStore = bbgo.NewOrderStore(s.Symbol)
	s.orderStore.BindStream(session.UserDataStream)

	s.tradeCollector = bbgo.NewTradeCollector(s.Symbol, s.Position, s.orderStore)

	s.tradeCollector.OnTrade(func(trade types.Trade, profit, netProfit fixedpoint.Value) {
		// StrategyController
		if s.Status != types.StrategyStatusRunning {
			return
		}

		s.Notifiability.Notify(trade)
		s.ProfitStats.AddTrade(trade)

		if profit.Compare(fixedpoint.Zero) == 0 {
			s.Environment.RecordPosition(s.Position, trade, nil)
		} else {
			log.Infof("%s generated profit: %v", s.Symbol, profit)
			p := s.Position.NewProfit(trade, profit, netProfit)
			p.Strategy = ID
			p.StrategyInstanceID = instanceID
			s.Notify(&p)

			s.ProfitStats.AddProfit(p)
			s.Notify(&s.ProfitStats)

			s.Environment.RecordPosition(s.Position, trade, &p)
		}
	})

	s.tradeCollector.OnPositionUpdate(func(position *types.Position) {
		log.Infof("position changed: %s", s.Position)
		s.Notify(s.Position)
	})

	s.tradeCollector.BindStream(session.UserDataStream)

	s.SmartStops.RunStopControllers(ctx, session, s.tradeCollector)

	session.UserDataStream.OnStart(func() {
		if s.UseTickerPrice {
			ticker, err := s.session.Exchange.QueryTicker(ctx, s.Symbol)
			if err != nil {
				return
			}

			midPrice := ticker.Buy.Add(ticker.Sell).Div(two)
			s.placeOrders(ctx, orderExecutor, midPrice, nil)
		} else {
			if price, ok := session.LastPrice(s.Symbol); ok {
				s.placeOrders(ctx, orderExecutor, price, nil)
			}
		}
	})

	session.MarketDataStream.OnKLineClosed(func(kline types.KLine) {
		// StrategyController
		if s.Status != types.StrategyStatusRunning {
			return
		}

		// Update spreads
		if s.DynamicSpreadWindow > 0 && kline.Direction() == types.DirectionUp {
			s.DynamicAskSpread.Update(kline.GetHigh().Sub(kline.GetOpen()).Div(kline.GetOpen()).Float64())
		}
		if s.DynamicSpreadWindow > 0 && kline.Direction() == types.DirectionDown {
			s.DynamicBidSpread.Update(kline.GetOpen().Sub(kline.GetLow()).Div(kline.GetOpen()).Float64())
		}
		if s.DynamicSpreadWindow > 0 && s.DynamicBidSpread.Length() >= s.DynamicSpreadWindow {
			dynamicBidSpread := fixedpoint.NewFromFloat(s.DynamicBidSpread.Last())
			if !s.MaxBidSpread.IsZero() && dynamicBidSpread.Compare(s.MaxBidSpread) > 0 {
				s.BidSpread = s.MaxBidSpread
			} else if !s.MinBidSpread.IsZero() && dynamicBidSpread.Compare(s.MinBidSpread) < 0 {
				s.BidSpread = s.MinBidSpread
			} else {
				s.BidSpread = dynamicBidSpread
			}
			log.Infof("new bid spread: %v", s.BidSpread.Percentage())
		}
		if s.DynamicSpreadWindow > 0 && s.DynamicAskSpread.Length() >= s.DynamicSpreadWindow {
			dynamicAskSpread := fixedpoint.NewFromFloat(s.DynamicAskSpread.Last())
			if !s.MaxAskSpread.IsZero() && dynamicAskSpread.Compare(s.MaxAskSpread) > 0 {
				s.AskSpread = s.MaxAskSpread
			} else if !s.MinAskSpread.IsZero() && dynamicAskSpread.Compare(s.MinAskSpread) < 0 {
				s.AskSpread = s.MinAskSpread
			} else {
				s.AskSpread = dynamicAskSpread
			}
			log.Infof("new ask spread: %v", s.AskSpread.Percentage())
		}

		if kline.Symbol != s.Symbol || kline.Interval != s.Interval {
			return
		}

		if err := s.activeMakerOrders.GracefulCancel(ctx, s.session.Exchange); err != nil {
			log.WithError(err).Errorf("graceful cancel order error")
		}

		// check if there is a canceled order had partially filled.
		s.tradeCollector.Process()

		if s.UseTickerPrice {
			ticker, err := s.session.Exchange.QueryTicker(ctx, s.Symbol)
			if err != nil {
				return
			}

			midPrice := ticker.Buy.Add(ticker.Sell).Div(two)
			log.Infof("using ticker price: bid %v / ask %v, mid price %v", ticker.Buy, ticker.Sell, midPrice)
			s.placeOrders(ctx, orderExecutor, midPrice, &kline)
		} else {
			s.placeOrders(ctx, orderExecutor, kline.Close, &kline)
		}
	})

	// s.book = types.NewStreamBook(s.Symbol)
	// s.book.BindStreamForBackground(session.MarketDataStream)

	s.Graceful.OnShutdown(func(ctx context.Context, wg *sync.WaitGroup) {
		defer wg.Done()
		close(s.stopC)

		if err := s.activeMakerOrders.GracefulCancel(ctx, s.session.Exchange); err != nil {
			log.WithError(err).Errorf("graceful cancel order error")
		}

		s.tradeCollector.Process()
	})

	return nil
}

func calculateBandPercentage(up, down, sma, midPrice float64) float64 {
	if midPrice < sma {
		// should be negative percentage
		return (midPrice - sma) / math.Abs(sma-down)
	} else if midPrice > sma {
		// should be positive percentage
		return (midPrice - sma) / math.Abs(up-sma)
	}

	return 0.0
}

func inBetween(x, a, b float64) bool {
	return a < x && x < b
}
