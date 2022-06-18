package rsmaker

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/c9s/bbgo/pkg/indicator"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/service"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/muesli/clusters"
	"github.com/muesli/kmeans"
)

// TODO:
// 1) add option for placing orders only when in neutral band
// 2) add option for only placing buy orders when price is below the SMA line

const ID = "rsmaker"

const stateKey = "state-v1"

var defaultFeeRate = fixedpoint.NewFromFloat(0.001)
var notionModifier = fixedpoint.NewFromFloat(1.1)
var two = fixedpoint.NewFromInt(2)

var log = logrus.WithField("strategy", ID)

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

type State struct {
	Position    *types.Position   `json:"position,omitempty"`
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
	status types.StrategyStatus
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) Initialize() error {
	return s.SmartStops.InitializeStopControllers(s.Symbol)
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{
		Interval: s.Interval,
	})

	//session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{
	//	Interval: types.Interval12h.String(),
	//})

	//if s.DefaultBollinger != nil && s.DefaultBollinger.Interval != "" {
	//	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{
	//		Interval: string(s.DefaultBollinger.Interval),
	//	})
	//}
	//
	//if s.NeutralBollinger != nil && s.NeutralBollinger.Interval != "" {
	//	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{
	//		Interval: string(s.NeutralBollinger.Interval),
	//	})
	//}

	//s.SmartStops.Subscribe(session)
}

func (s *Strategy) Validate() error {
	if len(s.Symbol) == 0 {
		return errors.New("symbol is required")
	}

	return nil
}

func (s *Strategy) CurrentPosition() *types.Position {
	return s.state.Position
}

func (s *Strategy) ClosePosition(ctx context.Context, percentage fixedpoint.Value) error {
	base := s.state.Position.GetBase()
	if base.IsZero() {
		return fmt.Errorf("no opened %s position", s.state.Position.Symbol)
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
	s.tradeCollector.Process()

	return err
}

// StrategyController

func (s *Strategy) GetStatus() types.StrategyStatus {
	return s.status
}

func (s *Strategy) Suspend(ctx context.Context) error {
	s.status = types.StrategyStatusStopped

	// Cancel all order
	if err := s.activeMakerOrders.GracefulCancel(ctx, s.session.Exchange); err != nil {
		log.WithError(err).Errorf("graceful cancel order error")
		s.Notify("graceful cancel order error")
	} else {
		s.Notify("All orders cancelled.")
	}

	s.tradeCollector.Process()

	// Save state
	if err := s.SaveState(); err != nil {
		log.WithError(err).Errorf("can not save state: %+v", s.state)
	} else {
		log.Infof("%s position is saved.", s.Symbol)
	}

	return nil
}

func (s *Strategy) Resume(ctx context.Context) error {
	s.status = types.StrategyStatusRunning

	return nil
}

//func (s *Strategy) EmergencyStop(ctx context.Context) error {
//	// Close 100% position
//	percentage, _ := fixedpoint.NewFromString("100%")
//	err := s.ClosePosition(ctx, percentage)
//
//	// Suspend strategy
//	_ = s.Suspend(ctx)
//
//	return err
//}

func (s *Strategy) SaveState() error {
	if err := s.Persistence.Save(s.state, ID, s.Symbol, stateKey); err != nil {
		return err
	}

	log.Infof("state is saved => %+v", s.state)
	return nil
}

func (s *Strategy) LoadState() error {
	var state State

	// load position
	if err := s.Persistence.Load(&state, ID, s.Symbol, stateKey); err != nil {
		if err != service.ErrPersistenceNotExists {
			return err
		}

		s.state = &State{}
	} else {
		s.state = &state
		log.Infof("state is restored: %+v", s.state)
	}

	// if position is nil, we need to allocate a new position for calculation
	if s.state.Position == nil {
		s.state.Position = types.NewPositionFromMarket(s.Market)
	}

	// init profit states
	s.state.ProfitStats.Symbol = s.Market.Symbol
	s.state.ProfitStats.BaseCurrency = s.Market.BaseCurrency
	s.state.ProfitStats.QuoteCurrency = s.Market.QuoteCurrency
	if s.state.ProfitStats.AccumulatedSince == 0 {
		s.state.ProfitStats.AccumulatedSince = time.Now().Unix()
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

func (s *Strategy) placeOrders(ctx context.Context, orderExecutor bbgo.OrderExecutor, midPrice fixedpoint.Value, klines []*types.KLine) {
	//bidSpread := s.Spread
	//if s.BidSpread.Sign() > 0 {
	//	bidSpread = s.BidSpread
	//}
	//
	//askSpread := s.Spread
	//if s.AskSpread.Sign() > 0 {
	//	askSpread = s.AskSpread
	//}
	// preprocessing
	max := 0.
	min := 100000.

	mv := 0.
	for x := 0; x < 50; x++ {
		if klines[x].High.Float64() > max {
			max = klines[x].High.Float64()
		}
		if klines[x].Low.Float64() < min {
			min = klines[x].High.Float64()
		}

		mv += klines[x].Volume.Float64()
	}
	mv = mv / 50

	//logrus.Info(max, min)
	// set up a random two-dimensional data set (float64 values between 0.0 and 1.0)
	var d clusters.Observations
	for x := 0; x < 50; x++ {
		//if klines[x].High.Float64() < max || klines[x].Low.Float64() > min {
		if klines[x].Volume.Float64() > mv*0.3 {
			d = append(d, clusters.Coordinates{
				klines[x].High.Float64(),
				klines[x].Low.Float64(),
				//klines[x].Open.Float64(),
				//klines[x].Close.Float64(),
				//klines[x].Volume.Float64(),
			})
		}
		//}

	}
	log.Info(len(d))

	// Partition the data points into 2 clusters
	km := kmeans.New()
	clusters, err := km.Partition(d, 3)

	//for _, c := range clusters {
	//fmt.Printf("Centered at x: %.2f y: %.2f\n", c.Center[0], c.Center[1])
	//fmt.Printf("Matching data points: %+v\n\n", c.Observations)
	//}
	// clustered virtual kline_1's mid price
	//vk1mp := fixedpoint.NewFromFloat((clusters[0].Center[0] + clusters[0].Center[1]) / 2.)
	// clustered virtual kline_2's mid price
	//vk2mp := fixedpoint.NewFromFloat((clusters[1].Center[0] + clusters[1].Center[1]) / 2.)
	// clustered virtual kline_3's mid price
	//vk3mp := fixedpoint.NewFromFloat((clusters[2].Center[0] + clusters[2].Center[1]) / 2.)

	// clustered virtual kline_1's high price
	vk1hp := fixedpoint.NewFromFloat(clusters[0].Center[0])
	// clustered virtual kline_2's high price
	vk2hp := fixedpoint.NewFromFloat(clusters[1].Center[0])
	// clustered virtual kline_3's high price
	vk3hp := fixedpoint.NewFromFloat(clusters[2].Center[0])

	// clustered virtual kline_1's low price
	vk1lp := fixedpoint.NewFromFloat(clusters[0].Center[1])
	// clustered virtual kline_2's low price
	vk2lp := fixedpoint.NewFromFloat(clusters[1].Center[1])
	// clustered virtual kline_3's low price
	vk3lp := fixedpoint.NewFromFloat(clusters[2].Center[1])

	askPrice := fixedpoint.NewFromFloat(math.Max(math.Max(vk1hp.Float64(), vk2hp.Float64()), vk3hp.Float64())) //fixedpoint.NewFromFloat(math.Max(math.Max(vk1mp.Float64(), vk2mp.Float64()), vk3mp.Float64()))
	bidPrice := fixedpoint.NewFromFloat(math.Min(math.Min(vk1lp.Float64(), vk2lp.Float64()), vk3lp.Float64())) //fixedpoint.NewFromFloat(math.Min(math.Min(vk1mp.Float64(), vk2mp.Float64()), vk3mp.Float64()))

	//if vk1mp.Compare(vk2mp) > 0 {
	//	askPrice = vk1mp //.Mul(fixedpoint.NewFromFloat(1.001))
	//	bidPrice = vk2mp //.Mul(fixedpoint.NewFromFloat(0.999))
	//} else if vk1mp.Compare(vk2mp) < 0 {
	//	askPrice = vk2mp //.Mul(fixedpoint.NewFromFloat(1.001))
	//	bidPrice = vk1mp //.Mul(fixedpoint.NewFromFloat(0.999))
	//}
	//midPrice.Mul(fixedpoint.One.Add(askSpread))
	//midPrice.Mul(fixedpoint.One.Sub(bidSpread))
	base := s.state.Position.GetBase()
	//balances := s.session.GetAccount().Balances()

	//log.Infof("mid price:%v spread: %s ask:%v bid: %v position: %s",
	//	midPrice,
	//	s.Spread.Percentage(),
	//	askPrice,
	//	bidPrice,
	//	s.state.Position,
	//)
	canSell := true
	canBuy := true

	//predMidPrice := (askPrice + bidPrice) / 2.

	//if midPrice.Float64() > predMidPrice.Float64() {
	//	bidPrice = predMidPrice.Mul(fixedpoint.NewFromFloat(0.999))
	//}
	//
	//if midPrice.Float64() < predMidPrice.Float64() {
	//	askPrice = predMidPrice.Mul(fixedpoint.NewFromFloat(1.001))
	//}
	//
	//if midPrice.Float64() > askPrice.Float64() {
	//	canBuy = false
	//	askPrice = midPrice.Mul(fixedpoint.NewFromFloat(1.001))
	//}
	//
	//if midPrice.Float64() < bidPrice.Float64() {
	//	canSell = false
	//	bidPrice = midPrice.Mul(fixedpoint.NewFromFloat(0.999))
	//}

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

	var submitBuyOrders []types.SubmitOrder
	var submitSellOrders []types.SubmitOrder

	//baseBalance, hasBaseBalance := balances[s.Market.BaseCurrency]
	//quoteBalance, hasQuoteBalance := balances[s.Market.QuoteCurrency]

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

	//if s.ShadowProtection && kline != nil {
	//	switch kline.Direction() {
	//	case types.DirectionDown:
	//		shadowHeight := kline.GetLowerShadowHeight()
	//		shadowRatio := kline.GetLowerShadowRatio()
	//		if shadowHeight.IsZero() && shadowRatio.Compare(s.ShadowProtectionRatio) < 0 {
	//			log.Infof("%s shadow protection enabled, lower shadow ratio %v < %v", s.Symbol, shadowRatio, s.ShadowProtectionRatio)
	//			canBuy = false
	//		}
	//	case types.DirectionUp:
	//		shadowHeight := kline.GetUpperShadowHeight()
	//		shadowRatio := kline.GetUpperShadowRatio()
	//		if shadowHeight.IsZero() || shadowRatio.Compare(s.ShadowProtectionRatio) < 0 {
	//			log.Infof("%s shadow protection enabled, upper shadow ratio %v < %v", s.Symbol, shadowRatio, s.ShadowProtectionRatio)
	//			canSell = false
	//		}
	//	}
	//}

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
	//if s.TradeInBand {
	//	if !inBetween(midPrice.Float64(), s.neutralBoll.LastDownBand(), s.neutralBoll.LastUpBand()) {
	//		log.Infof("tradeInBand is set, skip placing orders when the price is outside of the band")
	//		return
	//	}
	//}

	//revmacd := s.detectPriceTrend(s.neutralBoll, midPrice.Float64())
	//switch revmacd {
	//case NeutralTrend:
	//	// do nothing
	//
	//case UpTrend:
	//	skew := s.UptrendSkew
	//	buyOrder.Quantity = fixedpoint.Max(s.Market.MinQuantity, sellOrder.Quantity.Mul(skew))
	//
	//case DownTrend:
	//	skew := s.DowntrendSkew
	//	ratio := fixedpoint.One.Div(skew)
	//	sellOrder.Quantity = fixedpoint.Max(s.Market.MinQuantity, buyOrder.Quantity.Mul(ratio))
	//
	//}

	//if !hasQuoteBalance || buyOrder.Quantity.Mul(buyOrder.Price).Compare(quoteBalance.Available) > 0 {
	//	canBuy = false
	//}
	//
	//if !hasBaseBalance || sellOrder.Quantity.Compare(baseBalance.Available) > 0 {
	//	canSell = false
	//}

	//if midPrice.Compare(s.state.Position.AverageCost.Mul(fixedpoint.One.Add(s.MinProfitSpread))) < 0 {
	//	canSell = false
	//}

	//if s.Long != nil && *s.Long && base.Sub(sellOrder.Quantity).Sign() < 0 {
	//	canSell = false
	//}
	//
	//if s.BuyBelowNeutralSMA && midPrice.Float64() > s.neutralBoll.LastSMA() {
	//	canBuy = false
	//}

	if canSell {
		submitSellOrders = append(submitSellOrders, sellOrder)
		//sellOrder = s.adjustOrderPrice(sellOrder, false)
		//submitSellOrders = append(submitSellOrders, sellOrder)
		//sellOrder = s.adjustOrderPrice(sellOrder, false)
		//submitSellOrders = append(submitSellOrders, sellOrder)
	}
	if canBuy {
		submitBuyOrders = append(submitBuyOrders, buyOrder)
		//buyOrder = s.adjustOrderPrice(buyOrder, true)
		//submitBuyOrders = append(submitBuyOrders, buyOrder)
		//buyOrder = s.adjustOrderPrice(buyOrder, true)
		//submitBuyOrders = append(submitBuyOrders, buyOrder)
	}

	// condition for lower the average cost
	/*
		if midPrice < s.state.Position.AverageCost.MulFloat64(1.0-s.MinProfitSpread.Float64()) && canBuy {
			submitOrders = append(submitOrders, buyOrder)
		}
	*/

	for i := range submitBuyOrders {
		submitBuyOrders[i] = s.adjustOrderQuantity(submitBuyOrders[i])
	}

	for i := range submitSellOrders {
		submitSellOrders[i] = s.adjustOrderQuantity(submitSellOrders[i])
	}

	createdBuyOrders, err := orderExecutor.SubmitOrders(ctx, submitBuyOrders...)
	if err != nil {
		log.WithError(err).Errorf("can not place ping pong orders")
	}
	s.orderStore.Add(createdBuyOrders...)
	s.activeMakerOrders.Add(createdBuyOrders...)

	createdSellOrders, err := orderExecutor.SubmitOrders(ctx, submitSellOrders...)
	if err != nil {
		log.WithError(err).Errorf("can not place ping pong orders")
	}
	s.orderStore.Add(createdSellOrders...)
	s.activeMakerOrders.Add(createdSellOrders...)
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

func (s *Strategy) adjustOrderPrice(submitOrder types.SubmitOrder, side bool) types.SubmitOrder {

	if side {
		submitOrder.Price = submitOrder.Price.Mul(fixedpoint.NewFromFloat(0.995))
	} else {
		submitOrder.Price = submitOrder.Price.Mul(fixedpoint.NewFromFloat(1.005))
	}

	return submitOrder
}

func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	// StrategyController
	s.status = types.StrategyStatusRunning

	//if s.DisableShort {
	//	s.Long = &[]bool{true}[0]
	//}
	//
	//if s.MinProfitSpread.IsZero() {
	//	s.MinProfitSpread = fixedpoint.NewFromFloat(0.001)
	//}
	//
	//if s.UptrendSkew.IsZero() {
	//	s.UptrendSkew = fixedpoint.NewFromFloat(1.0 / 1.2)
	//}
	//
	//if s.DowntrendSkew.IsZero() {
	//	s.DowntrendSkew = fixedpoint.NewFromFloat(1.2)
	//}
	//
	//if s.ShadowProtectionRatio.IsZero() {
	//	s.ShadowProtectionRatio = fixedpoint.NewFromFloat(0.01)
	//}

	// initial required information
	s.session = session
	s.neutralBoll = s.StandardIndicatorSet.BOLL(s.NeutralBollinger.IntervalWindow, s.NeutralBollinger.BandWidth)
	s.defaultBoll = s.StandardIndicatorSet.BOLL(s.DefaultBollinger.IntervalWindow, s.DefaultBollinger.BandWidth)

	// calculate group id for orders
	instanceID := fmt.Sprintf("%s-%s", ID, s.Symbol)
	//s.groupID = max.GenerateGroupID(instanceID)
	log.Infof("using group id %d from fnv(%s)", s.groupID, instanceID)

	// restore state
	if err := s.LoadState(); err != nil {
		return err
	}

	s.state.Position.Strategy = ID
	s.state.Position.StrategyInstanceID = instanceID

	//s.stopC = make(chan struct{})

	s.activeMakerOrders = bbgo.NewLocalActiveOrderBook(s.Symbol)
	s.activeMakerOrders.BindStream(session.UserDataStream)

	s.orderStore = bbgo.NewOrderStore(s.Symbol)
	s.orderStore.BindStream(session.UserDataStream)

	s.tradeCollector = bbgo.NewTradeCollector(s.Symbol, s.state.Position, s.orderStore)

	//s.tradeCollector.OnTrade(func(trade types.Trade, profit, netProfit fixedpoint.Value) {
	//	// StrategyController
	//	if s.status != types.StrategyStatusRunning {
	//		return
	//	}
	//
	//	s.Notifiability.Notify(trade)
	//	s.state.ProfitStats.AddTrade(trade)
	//
	//	if profit.Compare(fixedpoint.Zero) == 0 {
	//		s.Environment.RecordPosition(s.state.Position, trade, nil)
	//	} else {
	//		log.Infof("%s generated profit: %v", s.Symbol, profit)
	//		p := s.state.Position.NewProfit(trade, profit, netProfit)
	//		p.Strategy = ID
	//		p.StrategyInstanceID = instanceID
	//		s.Notify(&p)
	//
	//		s.state.ProfitStats.AddProfit(p)
	//		s.Notify(&s.state.ProfitStats)
	//
	//		s.Environment.RecordPosition(s.state.Position, trade, &p)
	//	}
	//})
	//
	//s.tradeCollector.OnPositionUpdate(func(position *types.Position) {
	//	log.Infof("position changed: %s", s.state.Position)
	//	s.Notify(s.state.Position)
	//})

	s.tradeCollector.BindStream(session.UserDataStream)

	//s.SmartStops.RunStopControllers(ctx, session, s.tradeCollector)

	//session.UserDataStream.OnStart(func() {
	//if s.UseTickerPrice {
	//	ticker, err := s.session.Exchange.QueryTicker(ctx, s.Symbol)
	//	if err != nil {
	//		return
	//	}
	//
	//	midPrice := ticker.Buy.Add(ticker.Sell).Div(two)
	//	s.placeOrders(ctx, orderExecutor, midPrice, nil)
	//} else {
	//	if price, ok := session.LastPrice(s.Symbol); ok {
	//		s.placeOrders(ctx, orderExecutor, price, nil)
	//	}
	//}
	//})

	var klines []*types.KLine

	session.MarketDataStream.OnKLineClosed(func(kline types.KLine) {
		// StrategyController
		if s.status != types.StrategyStatusRunning {
			return
		}

		//if kline.Symbol != s.Symbol || kline.Interval != s.Interval {
		//	return
		//}

		if kline.Interval == s.Interval {
			klines = append(klines, &kline)
		}
		if len(klines) > 50 {
			//if s.UseTickerPrice {
			//	ticker, err := s.session.Exchange.QueryTicker(ctx, s.Symbol)
			//	if err != nil {
			//		return
			//	}
			//
			//	midPrice := ticker.Buy.Add(ticker.Sell).Div(two)
			//	log.Infof("using ticker price: bid %v / ask %v, mid price %v", ticker.Buy, ticker.Sell, midPrice)
			//	s.placeOrders(ctx, orderExecutor, midPrice, klines[len(klines)-100:])
			//	s.tradeCollector.Process()
			//}
			//else {
			if kline.Interval == s.Interval {

				//if s.state.Position.AverageCost.Div(kline.Close).Float64() < 0.999 {
				//	s.ClosePosition(ctx, fixedpoint.One)
				//	s.tradeCollector.Process()
				//}

				if err := s.activeMakerOrders.GracefulCancel(ctx, s.session.Exchange); err != nil {
					log.WithError(err).Errorf("graceful cancel order error")
				}

				// check if there is a canceled order had partially filled.
				s.tradeCollector.Process()

				s.placeOrders(ctx, orderExecutor, kline.Close, klines[len(klines)-50:])
				s.tradeCollector.Process()
			}
			//}
		}

	})

	// s.book = types.NewStreamBook(s.Symbol)
	// s.book.BindStreamForBackground(session.MarketDataStream)

	//s.Graceful.OnShutdown(func(ctx context.Context, wg *sync.WaitGroup) {
	//	//defer wg.Done()
	//	//close(s.stopC)
	//
	//	if err := s.activeMakerOrders.GracefulCancel(ctx, s.session.Exchange); err != nil {
	//		log.WithError(err).Errorf("graceful cancel order error")
	//	}
	//
	//	s.tradeCollector.Process()
	//
	//	if err := s.SaveState(); err != nil {
	//		log.WithError(err).Errorf("can not save state: %+v", s.state)
	//	}
	//})

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
