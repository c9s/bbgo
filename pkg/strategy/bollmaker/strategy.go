package bollmaker

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/c9s/bbgo/pkg/indicator"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/exchange/max"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/service"
	"github.com/c9s/bbgo/pkg/types"
)

// TODO:
// 1) add option for placing orders only when in neutral band
// 2) add option for only placing buy orders when price is below the SMA line

const ID = "bollmaker"

const stateKey = "state-v1"

var one = fixedpoint.NewFromFloat(1.0)

var defaultFeeRate = fixedpoint.NewFromFloat(0.001)

var log = logrus.WithField("strategy", ID)

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

type State struct {
	Position    *types.Position  `json:"position,omitempty"`
	ProfitStats bbgo.ProfitStats `json:"profitStats,omitempty"`
}

type BollingerSetting struct {
	types.IntervalWindow
	BandWidth float64 `json:"bandWidth"`
}

type TrailingStop struct {
	// CallbackRate is the callback rate from the previous high price
	CallbackRate fixedpoint.Value `json:"callbackRate,omitempty"`

	// ClosePosition is a percentage of the position to be closed
	ClosePosition fixedpoint.Value `json:"closePosition,omitempty"`

	// MinProfit is the percentage of the minimum profit ratio.
	// Stop order will be activiated only when the price reaches above this threshold.
	MinProfit fixedpoint.Value `json:"minProfit,omitempty"`

	// Interval is the time resolution to update the stop order
	// KLine per Interval will be used for updating the stop order
	Interval types.Interval `json:"interval,omitempty"`

	// Virtual is used when you don't want to place the real order on the exchange and lock the balance.
	// You want to handle the stop order by the strategy itself.
	Virtual bool `json:"virtual,omitempty"`
}

type TrailingStopController struct {
	*TrailingStop

	Symbol string

	position    *types.Position
	latestHigh  float64
	averageCost fixedpoint.Value
}

func NewTrailingStopController(symbol string, config *TrailingStop) *TrailingStopController {
	return &TrailingStopController{
		TrailingStop: config,
		Symbol:       symbol,
	}
}

func (c *TrailingStopController) Subscribe(session *bbgo.ExchangeSession) {
	session.Subscribe(types.KLineChannel, c.Symbol, types.SubscribeOptions{
		Interval: c.Interval.String(),
	})
}

func (c *TrailingStopController) Run(ctx context.Context, session *bbgo.ExchangeSession, tradeCollector *bbgo.TradeCollector) {
	// store the position
	c.position = tradeCollector.Position()
	c.averageCost = c.position.AverageCost

	// Use trade collector to get the position update event
	tradeCollector.OnPositionUpdate(func(position *types.Position) {
		// update average cost if we have it.
		c.averageCost = position.AverageCost
	})

	session.MarketDataStream.OnKLineClosed(func(kline types.KLine) {
		if kline.Symbol != c.Symbol || kline.Interval != c.Interval {
			return
		}

		closePrice := kline.Close

		// update the latest high
		c.latestHigh = math.Max(closePrice, c.latestHigh)

		if c.Virtual {
			// if average cost is updated, we can check min profit
			if c.averageCost == 0 {
				return
			}

			// skip dust position
			if c.position.Base.Abs().Float64() < c.position.Market.MinQuantity || c.position.Base.Abs().Float64()*closePrice < c.position.Market.MinNotional {
				return
			}

			// if it's in the callback rate, we don't want to trigger stop
			if closePrice < c.latestHigh && changeRate(closePrice, c.latestHigh) < c.CallbackRate.Float64() {
				return
			}

			// if the profit rate is defined, and it is less than our minimum profit rate, we skip stop
			if c.MinProfit > 0 &&
				(closePrice < c.averageCost.Float64() ||
					changeRate(closePrice, c.averageCost.Float64()) < c.MinProfit.Float64()) {
				return
			}

			marketOrder := c.position.NewClosePositionOrder(c.ClosePosition.Float64())
			if marketOrder != nil {
				log.Infof("trailing stop event emitted, latest high: %f, closed price: %f, average cost: %f, submitting market order to stop: %+v", c.latestHigh, closePrice, c.averageCost.Float64(), marketOrder)
				// skip dust order
				if marketOrder.Quantity*closePrice < c.position.Market.MinNotional {
					log.Warnf("market order quote quantity %f < min notional %f, skip placing order", marketOrder.Quantity*closePrice, c.position.Market.MinNotional)
					return
				}

				createdOrders, err := session.Exchange.SubmitOrders(ctx, *marketOrder)
				if err != nil {
					log.WithError(err).Errorf("stop market order place error")
					return
				}
				tradeCollector.OrderStore().Add(createdOrders...)
				tradeCollector.Process()

				// reset the state
				c.latestHigh = 0.0
			}
		} else {
			// place stop order only when the closed price is greater than the current average cost
			if c.position != nil && c.MinProfit > 0 && c.averageCost > 0 &&
				closePrice > c.averageCost.Float64() &&
				changeRate(closePrice, c.averageCost.Float64()) >= c.MinProfit.Float64() {

				stopPrice := c.averageCost.MulFloat64(1.0 + c.MinProfit.Float64())
				orderForm := c.GenerateStopOrder(stopPrice.Float64(), c.averageCost.Float64())
				if orderForm != nil {
					log.Infof("updating stop limit order to simulate trailing stop order...")
					createdOrders, err := session.Exchange.SubmitOrders(ctx, *orderForm)
					if err != nil {
						log.WithError(err).Errorf("stop order place error")
					}
					tradeCollector.OrderStore().Add(createdOrders...)
					tradeCollector.Process()
				}
			}
		}
	})
}

func (c *TrailingStopController) GenerateStopOrder(stopPrice, price float64) *types.SubmitOrder {
	base := c.position.GetBase()
	if base == 0 {
		return nil
	}

	quantity := math.Abs(base.Float64())
	quoteQuantity := price * quantity

	if c.ClosePosition > 0 {
		quantity = quantity * c.ClosePosition.Float64()
	}

	// skip dust orders
	if quantity < c.position.Market.MinQuantity || quoteQuantity < c.position.Market.MinNotional {
		return nil
	}

	side := types.SideTypeSell
	if base < 0 {
		side = types.SideTypeBuy
	}

	return &types.SubmitOrder{
		Symbol:    c.Symbol,
		Market:    c.position.Market,
		Type:      types.OrderTypeStopLimit,
		Side:      side,
		StopPrice: stopPrice,
		Price:     price,
		Quantity:  quantity,
	}
}

type FixedStop struct{}

type Stop struct {
	TrailingStop *TrailingStop `json:"trailingStop,omitempty"`
	FixedStop    *FixedStop    `json:"fixedStop,omitempty"`
}

type Strategy struct {
	*bbgo.Graceful
	*bbgo.Notifiability
	*bbgo.Persistence

	StandardIndicatorSet *bbgo.StandardIndicatorSet

	// Symbol is the market symbol you want to trade
	Symbol string `json:"symbol"`

	// Interval is how long do you want to update your order price and quantity
	Interval types.Interval `json:"interval"`

	QuantityOrAmount

	// Spread is the price spread from the middle price.
	// For ask orders, the ask price is ((bestAsk + bestBid) / 2 * (1.0 + spread))
	// For bid orders, the bid price is ((bestAsk + bestBid) / 2 * (1.0 - spread))
	// Spread can be set by percentage or floating number. e.g., 0.1% or 0.001
	Spread fixedpoint.Value `json:"spread"`

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

	Stops []Stop `json:"stops,omitempty"`

	session *bbgo.ExchangeSession
	book    *types.StreamOrderBook
	market  types.Market

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

	stopControllers []*TrailingStopController
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) Initialize() error {
	for _, stop := range s.Stops {
		s.stopControllers = append(s.stopControllers,
			NewTrailingStopController(s.Symbol, stop.TrailingStop),
		)
	}
	return nil
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	// session.Subscribe(types.BookChannel, s.Symbol, types.SubscribeOptions{})
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

	for _, stopController := range s.stopControllers {
		stopController.Subscribe(session)
	}
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

func (s *Strategy) ClosePosition(ctx context.Context, percentage float64) error {
	base := s.state.Position.GetBase()
	if base == 0 {
		return fmt.Errorf("no opened %s position", s.state.Position.Symbol)
	}

	// make it negative
	quantity := base.MulFloat64(percentage).Abs()
	side := types.SideTypeBuy
	if base > 0 {
		side = types.SideTypeSell
	}

	if quantity.Float64() < s.market.MinQuantity {
		return fmt.Errorf("order quantity %f is too small, less than %f", quantity.Float64(), s.market.MinQuantity)
	}

	submitOrder := types.SubmitOrder{
		Symbol:   s.Symbol,
		Side:     side,
		Type:     types.OrderTypeMarket,
		Quantity: quantity.Float64(),
		Market:   s.market,
	}

	s.Notify("Submitting %s %s order to close position by %f", s.Symbol, side.String(), percentage, submitOrder)

	createdOrders, err := s.session.Exchange.SubmitOrders(ctx, submitOrder)
	if err != nil {
		log.WithError(err).Errorf("can not place position close order")
	}

	s.orderStore.Add(createdOrders...)
	s.activeMakerOrders.Add(createdOrders...)
	return err
}

func (s *Strategy) SaveState() error {
	if err := s.Persistence.Save(s.state, ID, s.Symbol, stateKey); err != nil {
		return err
	} else {
		log.Infof("state is saved => %+v", s.state)
	}
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
		s.state.Position = types.NewPositionFromMarket(s.market)
	}

	// init profit states
	s.state.ProfitStats.Symbol = s.market.Symbol
	s.state.ProfitStats.BaseCurrency = s.market.BaseCurrency
	s.state.ProfitStats.QuoteCurrency = s.market.QuoteCurrency
	if s.state.ProfitStats.AccumulatedSince == 0 {
		s.state.ProfitStats.AccumulatedSince = time.Now().Unix()
	}

	return nil
}

func (s *Strategy) getCurrentAllowedExposurePosition(bandPercentage float64) (fixedpoint.Value, error) {
	if s.DynamicExposurePositionScale != nil {
		v, err := s.DynamicExposurePositionScale.Scale(bandPercentage)
		if err != nil {
			return 0, err
		}
		return fixedpoint.NewFromFloat(v), nil
	}

	return s.MaxExposurePosition, nil
}

func (s *Strategy) placeOrders(ctx context.Context, orderExecutor bbgo.OrderExecutor, midPrice fixedpoint.Value, kline *types.KLine) {
	askPrice := midPrice.Mul(one + s.Spread)
	bidPrice := midPrice.Mul(one - s.Spread)
	base := s.state.Position.GetBase()
	balances := s.session.Account.Balances()

	log.Infof("mid price:%f spread: %s ask:%f bid: %f position: %s",
		midPrice.Float64(),
		s.Spread.Percentage(),
		askPrice.Float64(),
		bidPrice.Float64(),
		s.state.Position.String(),
	)

	sellQuantity := s.CalculateQuantity(askPrice)
	buyQuantity := s.CalculateQuantity(bidPrice)

	sellOrder := types.SubmitOrder{
		Symbol:   s.Symbol,
		Side:     types.SideTypeSell,
		Type:     types.OrderTypeLimitMaker,
		Quantity: sellQuantity.Float64(),
		Price:    askPrice.Float64(),
		Market:   s.market,
		GroupID:  s.groupID,
	}
	buyOrder := types.SubmitOrder{
		Symbol:   s.Symbol,
		Side:     types.SideTypeBuy,
		Type:     types.OrderTypeLimitMaker,
		Quantity: buyQuantity.Float64(),
		Price:    bidPrice.Float64(),
		Market:   s.market,
		GroupID:  s.groupID,
	}

	var submitOrders []types.SubmitOrder

	baseBalance, hasBaseBalance := balances[s.market.BaseCurrency]
	quoteBalance, hasQuoteBalance := balances[s.market.QuoteCurrency]

	downBand := s.defaultBoll.LastDownBand()
	upBand := s.defaultBoll.LastUpBand()
	sma := s.defaultBoll.LastSMA()
	log.Infof("bollinger band: up %f sma %f down %f", upBand, sma, downBand)

	bandPercentage := calculateBandPercentage(upBand, downBand, sma, midPrice.Float64())
	log.Infof("mid price band percentage: %f", bandPercentage)

	maxExposurePosition, err := s.getCurrentAllowedExposurePosition(bandPercentage)
	if err != nil {
		log.WithError(err).Errorf("can not calculate CurrentAllowedExposurePosition")
		return
	}

	log.Infof("calculated max exposure position: %f", maxExposurePosition.Float64())

	canBuy := maxExposurePosition > 0 && base < maxExposurePosition
	canSell := maxExposurePosition > 0 && base > -maxExposurePosition

	if s.ShadowProtection && kline != nil {
		switch kline.Direction() {
		case types.DirectionDown:
			shadowHeight := kline.GetLowerShadowHeight()
			shadowRatio := kline.GetLowerShadowRatio()
			if shadowHeight == 0.0 && shadowRatio < s.ShadowProtectionRatio.Float64() {
				log.Infof("%s shadow protection enabled, lower shadow ratio %f < %f", s.Symbol, shadowRatio, s.ShadowProtectionRatio.Float64())
				canBuy = false
			}
		case types.DirectionUp:
			shadowHeight := kline.GetUpperShadowHeight()
			shadowRatio := kline.GetUpperShadowRatio()
			if shadowHeight == 0.0 || shadowRatio < s.ShadowProtectionRatio.Float64() {
				log.Infof("%s shadow protection enabled, upper shadow ratio %f < %f", s.Symbol, shadowRatio, s.ShadowProtectionRatio.Float64())
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
		skew := s.UptrendSkew.Float64()
		buyOrder.Quantity = math.Max(s.market.MinQuantity, sellOrder.Quantity*skew)

	case DownTrend:
		skew := s.DowntrendSkew.Float64()
		ratio := 1.0 / skew
		sellOrder.Quantity = math.Max(s.market.MinQuantity, buyOrder.Quantity*ratio)

	}

	if !hasQuoteBalance || (buyOrder.Quantity*buyOrder.Price) > quoteBalance.Available.Float64() {
		canBuy = false
	}

	if !hasBaseBalance || sellOrder.Quantity > baseBalance.Available.Float64() {
		canSell = false
	}

	if canSell && midPrice > s.state.Position.AverageCost.MulFloat64(1.0+s.MinProfitSpread.Float64()) {
		if !(s.DisableShort && (base.Float64()-sellOrder.Quantity < 0)) {
			submitOrders = append(submitOrders, sellOrder)
		}
	}
	if canBuy {
		submitOrders = append(submitOrders, buyOrder)
	}

	// condition for lower the average cost
	/*
		if midPrice < s.state.Position.AverageCost.MulFloat64(1.0-s.MinProfitSpread.Float64()) && canBuy {
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
	if submitOrder.Quantity*submitOrder.Price < s.market.MinNotional {
		submitOrder.Quantity = bbgo.AdjustFloatQuantityByMinAmount(submitOrder.Quantity, submitOrder.Price, s.market.MinNotional*1.1)
	}

	if submitOrder.Quantity < s.market.MinQuantity {
		submitOrder.Quantity = math.Max(submitOrder.Quantity, s.market.MinQuantity)
	}

	return submitOrder
}

func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	if s.MinProfitSpread == 0 {
		s.MinProfitSpread = fixedpoint.NewFromFloat(0.001)
	}

	if s.UptrendSkew == 0 {
		s.UptrendSkew = fixedpoint.NewFromFloat(1.0 / 1.2)
	}

	if s.DowntrendSkew == 0 {
		s.DowntrendSkew = fixedpoint.NewFromFloat(1.2)
	}

	if s.ShadowProtectionRatio == 0 {
		s.ShadowProtectionRatio = fixedpoint.NewFromFloat(0.01)
	}

	// initial required information
	s.session = session

	market, ok := session.Market(s.Symbol)
	if !ok {
		return fmt.Errorf("market %s not found", s.Symbol)
	}
	s.market = market

	s.neutralBoll = s.StandardIndicatorSet.BOLL(s.NeutralBollinger.IntervalWindow, s.NeutralBollinger.BandWidth)
	s.defaultBoll = s.StandardIndicatorSet.BOLL(s.DefaultBollinger.IntervalWindow, s.DefaultBollinger.BandWidth)

	// calculate group id for orders
	instanceID := fmt.Sprintf("%s-%s", ID, s.Symbol)
	s.groupID = max.GenerateGroupID(instanceID)
	log.Infof("using group id %d from fnv(%s)", s.groupID, instanceID)

	// restore state
	if err := s.LoadState(); err != nil {
		return err
	}

	s.stopC = make(chan struct{})

	s.activeMakerOrders = bbgo.NewLocalActiveOrderBook(s.Symbol)
	s.activeMakerOrders.BindStream(session.UserDataStream)

	s.orderStore = bbgo.NewOrderStore(s.Symbol)
	s.orderStore.BindStream(session.UserDataStream)

	s.tradeCollector = bbgo.NewTradeCollector(s.Symbol, s.state.Position, s.orderStore)
	s.tradeCollector.OnProfit(func(trade types.Trade, profit fixedpoint.Value, netProfit fixedpoint.Value) {
		p := bbgo.Profit{
			Symbol:          s.Symbol,
			Profit:          profit,
			NetProfit:       netProfit,
			TradeAmount:     fixedpoint.NewFromFloat(trade.QuoteQuantity),
			ProfitMargin:    profit.DivFloat64(trade.QuoteQuantity),
			NetProfitMargin: netProfit.DivFloat64(trade.QuoteQuantity),
			QuoteCurrency:   s.state.Position.QuoteCurrency,
			BaseCurrency:    s.state.Position.BaseCurrency,
			Time:            trade.Time.Time(),
		}
		s.state.ProfitStats.AddProfit(p)
		s.Notify(&p)
		s.Notify(&s.state.ProfitStats)
	})

	s.tradeCollector.OnTrade(func(trade types.Trade) {
		s.Notifiability.Notify(trade)
		s.state.ProfitStats.AddTrade(trade)
	})

	s.tradeCollector.OnPositionUpdate(func(position *types.Position) {
		log.Infof("position changed: %s", s.state.Position)
		s.Notify(s.state.Position)
	})

	s.tradeCollector.BindStream(session.UserDataStream)

	for _, stopController := range s.stopControllers {
		stopController.Run(ctx, session, s.tradeCollector)
	}

	session.UserDataStream.OnStart(func() {
		if s.UseTickerPrice {
			ticker, err := s.session.Exchange.QueryTicker(ctx, s.Symbol)
			if err != nil {
				return
			}

			midPrice := fixedpoint.NewFromFloat((ticker.Buy + ticker.Sell) / 2)
			s.placeOrders(ctx, orderExecutor, midPrice, nil)
		} else {
			if price, ok := session.LastPrice(s.Symbol); ok {
				s.placeOrders(ctx, orderExecutor, fixedpoint.NewFromFloat(price), nil)
			}
		}
	})

	session.MarketDataStream.OnKLineClosed(func(kline types.KLine) {
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

			mid := (ticker.Buy + ticker.Sell) / 2
			log.Infof("using ticker price: bid %f / ask %f, mid price %f", ticker.Buy, ticker.Sell, mid)
			midPrice := fixedpoint.NewFromFloat(mid)
			s.placeOrders(ctx, orderExecutor, midPrice, &kline)
		} else {
			s.placeOrders(ctx, orderExecutor, fixedpoint.NewFromFloat(kline.Close), &kline)
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

		if err := s.SaveState(); err != nil {
			log.WithError(err).Errorf("can not save state: %+v", s.state)
		}
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

func changeRate(a, b float64) float64 {
	return math.Abs(a-b) / b
}
