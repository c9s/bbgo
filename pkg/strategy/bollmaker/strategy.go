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
	Environment          *bbgo.Environment
	StandardIndicatorSet *bbgo.StandardIndicatorSet
	Market               types.Market

	// Symbol is the market symbol you want to trade
	Symbol string `json:"symbol"`

	types.IntervalWindow

	bbgo.QuantityOrAmount

	// TrendEMA is used for detecting the trend by a given EMA
	// you can define interval and window
	TrendEMA *bbgo.TrendEMA `json:"trendEMA"`

	// Spread is the price spread from the middle price.
	// For ask orders, the ask price is ((bestAsk + bestBid) / 2 * (1.0 + spread))
	// For bid orders, the bid price is ((bestAsk + bestBid) / 2 * (1.0 - spread))
	// Spread can be set by percentage or floating number. e.g., 0.1% or 0.001
	Spread fixedpoint.Value `json:"spread"`

	// BidSpread overrides the spread setting, this spread will be used for the buy order
	BidSpread fixedpoint.Value `json:"bidSpread,omitempty"`

	// AskSpread overrides the spread setting, this spread will be used for the sell order
	AskSpread fixedpoint.Value `json:"askSpread,omitempty"`

	// DynamicSpread enables the automatic adjustment to bid and ask spread.
	DynamicSpread DynamicSpreadSettings `json:"dynamicSpread,omitempty"`

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

	session *bbgo.ExchangeSession
	book    *types.StreamOrderBook

	ExitMethods bbgo.ExitMethodSet `json:"exits"`

	// persistence fields
	Position    *types.Position    `json:"position,omitempty" persistence:"position"`
	ProfitStats *types.ProfitStats `json:"profitStats,omitempty" persistence:"profit_stats"`

	orderExecutor *bbgo.GeneralOrderExecutor

	groupID uint32

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

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{
		Interval: s.Interval,
	})

	if s.DefaultBollinger != nil && s.DefaultBollinger.Interval != "" {
		session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{
			Interval: s.DefaultBollinger.Interval,
		})
	}

	if s.NeutralBollinger != nil && s.NeutralBollinger.Interval != "" {
		session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{
			Interval: s.NeutralBollinger.Interval,
		})
	}

	if s.TrendEMA != nil {
		session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.TrendEMA.Interval})
	}

	s.ExitMethods.SetAndSubscribe(session, s)
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
	return s.orderExecutor.ClosePosition(ctx, percentage)
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

func (s *Strategy) placeOrders(ctx context.Context, midPrice fixedpoint.Value, kline *types.KLine) {
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

	downBand := s.defaultBoll.DownBand.Last()
	upBand := s.defaultBoll.UpBand.Last()
	sma := s.defaultBoll.SMA.Last()
	log.Infof("%s bollinger band: up %f sma %f down %f", s.Symbol, upBand, sma, downBand)

	bandPercentage := calculateBandPercentage(upBand, downBand, sma, midPrice.Float64())
	log.Infof("%s mid price band percentage: %v", s.Symbol, bandPercentage)

	maxExposurePosition, err := s.getCurrentAllowedExposurePosition(bandPercentage)
	if err != nil {
		log.WithError(err).Errorf("can not calculate %s CurrentAllowedExposurePosition", s.Symbol)
		return
	}

	log.Infof("calculated %s max exposure position: %v", s.Symbol, maxExposurePosition)

	if !s.Position.IsClosed() && !s.Position.IsDust(midPrice) {
		log.Infof("current %s unrealized profit: %f %s", s.Symbol, s.Position.UnrealizedProfit(midPrice).Float64(), s.Market.QuoteCurrency)
	}

	// by default, we turn both sell and buy on,
	// which means we will place buy and sell orders
	canSell := true
	canBuy := true

	if maxExposurePosition.Sign() > 0 && base.Compare(maxExposurePosition) > 0 {
		canBuy = false
	}

	if maxExposurePosition.Sign() > 0 {
		if s.hasLongSet() && base.Sign() < 0 {
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
		if !inBetween(midPrice.Float64(), s.neutralBoll.DownBand.Last(), s.neutralBoll.UpBand.Last()) {
			log.Infof("tradeInBand is set, skip placing orders when the price is outside of the band")
			return
		}
	}

	trend := detectPriceTrend(s.neutralBoll, midPrice.Float64())
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

	// check balance and switch the orders
	if !hasQuoteBalance || buyOrder.Quantity.Mul(buyOrder.Price).Compare(quoteBalance.Available) > 0 {
		canBuy = false
	}

	if !hasBaseBalance || sellOrder.Quantity.Compare(baseBalance.Available) > 0 {
		canSell = false
	}

	isLongPosition := s.Position.IsLong()
	isShortPosition := s.Position.IsShort()
	minProfitPrice := s.Position.AverageCost.Mul(fixedpoint.One.Add(s.MinProfitSpread))
	if isShortPosition {
		minProfitPrice = s.Position.AverageCost.Mul(fixedpoint.One.Sub(s.MinProfitSpread))
	}

	if isLongPosition {
		// for long position if the current price is lower than the minimal profitable price then we should stop sell
		// this avoid loss trade
		if midPrice.Compare(minProfitPrice) < 0 {
			canSell = false
		}
	} else if isShortPosition {
		// for short position if the current price is higher than the minimal profitable price then we should stop buy
		// this avoid loss trade
		if midPrice.Compare(minProfitPrice) > 0 {
			canBuy = false
		}
	}

	if s.hasLongSet() && base.Sub(sellOrder.Quantity).Sign() < 0 {
		canSell = false
	}

	if s.BuyBelowNeutralSMA && midPrice.Float64() > s.neutralBoll.SMA.Last() {
		canBuy = false
	}

	// trend EMA protection
	if s.TrendEMA != nil {
		if !s.TrendEMA.GradientAllowed() {
			log.Infof("trendEMA protection: midPrice price %f, gradient %f, turning buy order off", midPrice.Float64(), s.TrendEMA.Gradient())
			canBuy = false
		}
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
		submitOrders[i] = adjustOrderQuantity(submitOrders[i], s.Market)
	}

	_, _ = s.orderExecutor.SubmitOrders(ctx, submitOrders...)
}

func (s *Strategy) hasLongSet() bool {
	return s.Long != nil && *s.Long
}

func (s *Strategy) hasShortSet() bool {
	return s.Short != nil && *s.Short
}

func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	// initial required information
	s.session = session

	// StrategyController
	s.Status = types.StrategyStatusRunning

	s.neutralBoll = s.StandardIndicatorSet.BOLL(s.NeutralBollinger.IntervalWindow, s.NeutralBollinger.BandWidth)
	s.defaultBoll = s.StandardIndicatorSet.BOLL(s.DefaultBollinger.IntervalWindow, s.DefaultBollinger.BandWidth)

	// Setup dynamic spread
	if s.DynamicSpread.IsEnabled() {
		if s.DynamicSpread.Interval == "" {
			s.DynamicSpread.Interval = s.Interval
		}
		s.DynamicSpread.Initialize(s.Symbol, s.session, s.neutralBoll, s.defaultBoll)
	}

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

	// calculate group id for orders
	instanceID := s.InstanceID()
	s.groupID = util.FNV32(instanceID)

	// If position is nil, we need to allocate a new position for calculation
	if s.Position == nil {
		s.Position = types.NewPositionFromMarket(s.Market)
	}

	if s.session.MakerFeeRate.Sign() > 0 || s.session.TakerFeeRate.Sign() > 0 {
		s.Position.SetExchangeFeeRate(s.session.ExchangeName, types.ExchangeFee{
			MakerFeeRate: s.session.MakerFeeRate,
			TakerFeeRate: s.session.TakerFeeRate,
		})
	}

	if s.ProfitStats == nil {
		s.ProfitStats = types.NewProfitStats(s.Market)
	}

	// Always update the position fields
	s.Position.Strategy = ID
	s.Position.StrategyInstanceID = instanceID

	s.orderExecutor = bbgo.NewGeneralOrderExecutor(session, s.Symbol, ID, instanceID, s.Position)
	s.orderExecutor.BindEnvironment(s.Environment)
	s.orderExecutor.BindProfitStats(s.ProfitStats)
	s.orderExecutor.Bind()
	s.orderExecutor.TradeCollector().OnPositionUpdate(func(position *types.Position) {
		bbgo.Sync(ctx, s)
	})
	s.ExitMethods.Bind(session, s.orderExecutor)

	if s.TrendEMA != nil {
		s.TrendEMA.Bind(session, s.orderExecutor)
	}

	if bbgo.IsBackTesting {
		log.Warn("turning of useTickerPrice option in the back-testing environment...")
		s.UseTickerPrice = false
	}

	s.OnSuspend(func() {
		_ = s.orderExecutor.GracefulCancel(ctx)
		bbgo.Sync(ctx, s)
	})

	s.OnEmergencyStop(func() {
		// Close 100% position
		percentage := fixedpoint.NewFromFloat(1.0)
		_ = s.ClosePosition(ctx, percentage)
	})

	session.UserDataStream.OnStart(func() {
		if s.UseTickerPrice {
			ticker, err := s.session.Exchange.QueryTicker(ctx, s.Symbol)
			if err != nil {
				return
			}

			midPrice := ticker.Buy.Add(ticker.Sell).Div(two)
			s.placeOrders(ctx, midPrice, nil)
		} else {
			if price, ok := session.LastPrice(s.Symbol); ok {
				s.placeOrders(ctx, price, nil)
			}
		}
	})

	session.MarketDataStream.OnKLineClosed(types.KLineWith(s.Symbol, s.Interval, func(kline types.KLine) {
		// StrategyController
		if s.Status != types.StrategyStatusRunning {
			return
		}

		// Update spreads with dynamic spread
		if s.DynamicSpread.IsEnabled() {
			s.DynamicSpread.Update(kline)
			dynamicBidSpread, err := s.DynamicSpread.GetBidSpread()
			if err == nil && dynamicBidSpread > 0 {
				s.BidSpread = fixedpoint.NewFromFloat(dynamicBidSpread)
				log.Infof("%s dynamic bid spread updated: %s", s.Symbol, s.BidSpread.Percentage())
			}
			dynamicAskSpread, err := s.DynamicSpread.GetAskSpread()
			if err == nil && dynamicAskSpread > 0 {
				s.AskSpread = fixedpoint.NewFromFloat(dynamicAskSpread)
				log.Infof("%s dynamic ask spread updated: %s", s.Symbol, s.AskSpread.Percentage())
			}
		}

		_ = s.orderExecutor.GracefulCancel(ctx)

		if s.UseTickerPrice {
			ticker, err := s.session.Exchange.QueryTicker(ctx, s.Symbol)
			if err != nil {
				return
			}

			midPrice := ticker.Buy.Add(ticker.Sell).Div(two)
			log.Infof("using ticker price: bid %v / ask %v, mid price %v", ticker.Buy, ticker.Sell, midPrice)
			s.placeOrders(ctx, midPrice, &kline)
		} else {
			s.placeOrders(ctx, kline.Close, &kline)
		}
	}))

	// s.book = types.NewStreamBook(s.Symbol)
	// s.book.BindStreamForBackground(session.MarketDataStream)

	bbgo.OnShutdown(ctx, func(ctx context.Context, wg *sync.WaitGroup) {
		defer wg.Done()

		_ = s.orderExecutor.GracefulCancel(ctx)
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

func adjustOrderQuantity(submitOrder types.SubmitOrder, market types.Market) types.SubmitOrder {
	if submitOrder.Quantity.Mul(submitOrder.Price).Compare(market.MinNotional) < 0 {
		submitOrder.Quantity = bbgo.AdjustFloatQuantityByMinAmount(submitOrder.Quantity, submitOrder.Price, market.MinNotional.Mul(notionModifier))
	}

	if submitOrder.Quantity.Compare(market.MinQuantity) < 0 {
		submitOrder.Quantity = fixedpoint.Max(submitOrder.Quantity, market.MinQuantity)
	}

	return submitOrder
}
