package linregmaker

import (
	"context"
	"fmt"
	"github.com/c9s/bbgo/pkg/report"
	"os"
	"strconv"
	"sync"

	"github.com/c9s/bbgo/pkg/risk/dynamicrisk"

	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/util"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

// TODO: Docs

const ID = "linregmaker"

var notionModifier = fixedpoint.NewFromFloat(1.1)
var two = fixedpoint.NewFromInt(2)

var log = logrus.WithField("strategy", ID)

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

type Strategy struct {
	// Symbol is the market symbol you want to trade
	Symbol string `json:"symbol"`

	// Leverage uses the account net value to calculate the allowed margin
	Leverage fixedpoint.Value `json:"leverage"`

	types.IntervalWindow

	// ReverseEMA is used to determine the long-term trend.
	// Above the ReverseEMA is the long trend and vise versa.
	// All the opposite trend position will be closed upon the trend change
	ReverseEMA *indicator.EWMA `json:"reverseEMA"`

	// ReverseInterval is the interval to check trend reverse against ReverseEMA. Close price of this interval crossing
	// the ReverseEMA triggers main trend change.
	ReverseInterval types.Interval `json:"reverseInterval"`

	// mainTrendCurrent is the current long-term trend
	mainTrendCurrent types.Direction
	// mainTrendPrevious is the long-term trend of previous kline
	mainTrendPrevious types.Direction

	// FastLinReg is to determine the short-term trend.
	// Buy/sell orders are placed if the FastLinReg and the ReverseEMA trend are in the same direction, and only orders
	// that reduce position are placed if the FastLinReg and the ReverseEMA trend are in different directions.
	FastLinReg *indicator.LinReg `json:"fastLinReg"`

	// SlowLinReg is to determine the midterm trend.
	// When the SlowLinReg and the ReverseEMA trend are in different directions, creation of opposite position is
	// allowed.
	SlowLinReg *indicator.LinReg `json:"slowLinReg"`

	// AllowOppositePosition if true, the creation of opposite position is allowed when both fast and slow LinReg are in
	// the opposite direction to main trend
	AllowOppositePosition bool `json:"allowOppositePosition"`

	// FasterDecreaseRatio the quantity of decreasing position orders are multiplied by this ratio when both fast and
	// slow LinReg are in the opposite direction to main trend
	FasterDecreaseRatio fixedpoint.Value `json:"fasterDecreaseRatio,omitempty"`

	// NeutralBollinger is the smaller range of the bollinger band
	// If price is in this band, it usually means the price is oscillating.
	// If price goes out of this band, we tend to not place sell orders or buy orders
	NeutralBollinger types.IntervalWindowBandWidth `json:"neutralBollinger"`

	// neutralBoll is the neutral price section for TradeInBand
	neutralBoll *indicator.BOLL

	// TradeInBand
	// When this is on, places orders only when the current price is in the bollinger band.
	TradeInBand bool `json:"tradeInBand"`

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
	// Overrides Spread, BidSpread, and AskSpread
	DynamicSpread dynamicrisk.DynamicSpread `json:"dynamicSpread,omitempty"`

	// MaxExposurePosition is the maximum position you can hold
	// 10 means you can hold 10 ETH long/short position by maximum
	MaxExposurePosition fixedpoint.Value `json:"maxExposurePosition"`

	// DynamicExposure is used to define the exposure position range with the given percentage.
	// When DynamicExposure is set, your MaxExposurePosition will be calculated dynamically
	DynamicExposure dynamicrisk.DynamicExposure `json:"dynamicExposure"`

	bbgo.QuantityOrAmount

	// DynamicQuantityIncrease calculates the increase position order quantity dynamically
	DynamicQuantityIncrease dynamicrisk.DynamicQuantitySet `json:"dynamicQuantityIncrease"`

	// DynamicQuantityDecrease calculates the decrease position order quantity dynamically
	DynamicQuantityDecrease dynamicrisk.DynamicQuantitySet `json:"dynamicQuantityDecrease"`

	// UseDynamicQuantityAsAmount calculates amount instead of quantity
	UseDynamicQuantityAsAmount bool `json:"useDynamicQuantityAsAmount"`

	// MinProfitSpread is the minimal order price spread from the current average cost.
	// For long position, you will only place sell order above the price (= average cost * (1 + minProfitSpread))
	// For short position, you will only place buy order below the price (= average cost * (1 - minProfitSpread))
	MinProfitSpread fixedpoint.Value `json:"minProfitSpread"`

	// MinProfitActivationRate activates MinProfitSpread when position RoI higher than the specified percentage
	MinProfitActivationRate fixedpoint.Value `json:"minProfitActivationRate"`

	// ExitMethods are various TP/SL methods
	ExitMethods bbgo.ExitMethodSet `json:"exits"`

	// persistence fields
	Position    *types.Position    `persistence:"position"`
	ProfitStats *types.ProfitStats `persistence:"profit_stats"`
	TradeStats  *types.TradeStats  `persistence:"trade_stats"`

	// ProfitStatsTracker tracks profit related status and generates report
	ProfitStatsTracker *report.ProfitStatsTracker `json:"profitStatsTracker"`
	TrackParameters    bool                       `json:"trackParameters"`

	Environment          *bbgo.Environment
	StandardIndicatorSet *bbgo.StandardIndicatorSet
	Market               types.Market
	ctx                  context.Context

	session *bbgo.ExchangeSession

	orderExecutor *bbgo.GeneralOrderExecutor

	groupID uint32

	// StrategyController
	bbgo.StrategyController
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) InstanceID() string {
	return fmt.Sprintf("%s:%s", ID, s.Symbol)
}

// Validate basic config parameters
func (s *Strategy) Validate() error {
	if len(s.Symbol) == 0 {
		return errors.New("symbol is required")
	}

	if len(s.Interval) == 0 {
		return errors.New("interval is required")
	}

	if s.ReverseEMA == nil {
		return errors.New("reverseEMA must be set")
	}

	if s.FastLinReg == nil {
		return errors.New("fastLinReg must be set")
	}

	if s.SlowLinReg == nil {
		return errors.New("slowLinReg must be set")
	}

	return nil
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	// Subscribe for ReverseEMA
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{
		Interval: s.ReverseEMA.Interval,
	})

	// Subscribe for ReverseInterval. Use interval of ReverseEMA if ReverseInterval is omitted
	if s.ReverseInterval == "" {
		s.ReverseInterval = s.ReverseEMA.Interval
	}
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{
		Interval: s.ReverseInterval,
	})

	// Subscribe for LinRegs
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{
		Interval: s.FastLinReg.Interval,
	})
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{
		Interval: s.SlowLinReg.Interval,
	})
	// Initialize LinRegs
	kLineStore, _ := session.MarketDataStore(s.Symbol)
	s.FastLinReg.BindK(session.MarketDataStream, s.Symbol, s.FastLinReg.Interval)
	if klines, ok := kLineStore.KLinesOfInterval(s.FastLinReg.Interval); ok {
		s.FastLinReg.LoadK((*klines)[0:])
	}
	s.SlowLinReg.BindK(session.MarketDataStream, s.Symbol, s.SlowLinReg.Interval)
	if klines, ok := kLineStore.KLinesOfInterval(s.SlowLinReg.Interval); ok {
		s.SlowLinReg.LoadK((*klines)[0:])
	}

	// Subscribe for BBs
	if s.NeutralBollinger.Interval != "" {
		session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{
			Interval: s.NeutralBollinger.Interval,
		})
	}

	// Initialize Exits
	s.ExitMethods.SetAndSubscribe(session, s)

	// Initialize dynamic spread
	if s.DynamicSpread.IsEnabled() {
		s.DynamicSpread.Initialize(s.Symbol, session)
	}

	// Initialize dynamic exposure
	if s.DynamicExposure.IsEnabled() {
		s.DynamicExposure.Initialize(s.Symbol, session)
	}

	// Initialize dynamic quantities
	if len(s.DynamicQuantityIncrease) > 0 {
		s.DynamicQuantityIncrease.Initialize(s.Symbol, session)
	}
	if len(s.DynamicQuantityDecrease) > 0 {
		s.DynamicQuantityDecrease.Initialize(s.Symbol, session)
	}

	// Profit tracker
	if s.ProfitStatsTracker != nil {
		s.ProfitStatsTracker.Subscribe(session, s.Symbol)
	}
}

func (s *Strategy) CurrentPosition() *types.Position {
	return s.Position
}

func (s *Strategy) ClosePosition(ctx context.Context, percentage fixedpoint.Value) error {
	return s.orderExecutor.ClosePosition(ctx, percentage)
}

// isAllowOppositePosition returns if opening opposite position is allowed
func (s *Strategy) isAllowOppositePosition() bool {
	if !s.AllowOppositePosition {
		return false
	}

	if (s.mainTrendCurrent == types.DirectionUp && s.FastLinReg.Last(0) < 0 && s.SlowLinReg.Last(0) < 0) ||
		(s.mainTrendCurrent == types.DirectionDown && s.FastLinReg.Last(0) > 0 && s.SlowLinReg.Last(0) > 0) {
		log.Infof("%s allow opposite position is enabled: MainTrend %v, FastLinReg: %f, SlowLinReg: %f", s.Symbol, s.mainTrendCurrent, s.FastLinReg.Last(0), s.SlowLinReg.Last(0))
		return true
	}
	log.Infof("%s allow opposite position is disabled: MainTrend %v, FastLinReg: %f, SlowLinReg: %f", s.Symbol, s.mainTrendCurrent, s.FastLinReg.Last(0), s.SlowLinReg.Last(0))

	return false
}

// updateSpread for ask and bid price
func (s *Strategy) updateSpread() {
	// Update spreads with dynamic spread
	if s.DynamicSpread.IsEnabled() {
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

	if s.BidSpread.Sign() <= 0 {
		s.BidSpread = s.Spread
	}

	if s.BidSpread.Sign() <= 0 {
		s.AskSpread = s.Spread
	}
}

// updateMaxExposure with dynamic exposure
func (s *Strategy) updateMaxExposure(midPrice fixedpoint.Value) {
	// Calculate max exposure
	if s.DynamicExposure.IsEnabled() {
		var err error
		maxExposurePosition, err := s.DynamicExposure.GetMaxExposure(midPrice.Float64(), s.mainTrendCurrent)
		if err != nil {
			log.WithError(err).Errorf("can not calculate DynamicExposure of %s, use previous MaxExposurePosition instead", s.Symbol)
			bbgo.Notify("can not calculate DynamicExposure of %s, use previous MaxExposurePosition instead", s.Symbol)
		} else {
			s.MaxExposurePosition = maxExposurePosition
		}
		log.Infof("calculated %s max exposure position: %v", s.Symbol, s.MaxExposurePosition)
	}
}

// getOrderPrices returns ask and bid prices
func (s *Strategy) getOrderPrices(midPrice fixedpoint.Value) (askPrice fixedpoint.Value, bidPrice fixedpoint.Value) {
	askPrice = midPrice.Mul(fixedpoint.One.Add(s.AskSpread))
	bidPrice = midPrice.Mul(fixedpoint.One.Sub(s.BidSpread))
	log.Infof("%s mid price:%v ask:%v bid: %v", s.Symbol, midPrice, askPrice, bidPrice)

	return askPrice, bidPrice
}

// adjustQuantity to meet the min notional and qty requirement
func (s *Strategy) adjustQuantity(quantity, price fixedpoint.Value) fixedpoint.Value {
	adjustedQty := quantity
	if quantity.Mul(price).Compare(s.Market.MinNotional) < 0 {
		adjustedQty = bbgo.AdjustFloatQuantityByMinAmount(quantity, price, s.Market.MinNotional.Mul(notionModifier))
	}

	if adjustedQty.Compare(s.Market.MinQuantity) < 0 {
		adjustedQty = fixedpoint.Max(adjustedQty, s.Market.MinQuantity)
	}

	return adjustedQty
}

// getOrderQuantities returns sell and buy qty
func (s *Strategy) getOrderQuantities(askPrice fixedpoint.Value, bidPrice fixedpoint.Value) (sellQuantity fixedpoint.Value, buyQuantity fixedpoint.Value) {
	// Default
	sellQuantity = s.QuantityOrAmount.CalculateQuantity(askPrice)
	buyQuantity = s.QuantityOrAmount.CalculateQuantity(bidPrice)

	// Dynamic qty
	switch {
	case s.mainTrendCurrent == types.DirectionUp:
		if len(s.DynamicQuantityIncrease) > 0 {
			qty, err := s.DynamicQuantityIncrease.GetQuantity(false)
			if err == nil {
				buyQuantity = qty
			} else {
				log.WithError(err).Errorf("cannot get dynamic buy qty of %s, use default qty instead", s.Symbol)
				bbgo.Notify("cannot get dynamic buy qty of %s, use default qty instead", s.Symbol)
			}
		}
		if len(s.DynamicQuantityDecrease) > 0 {
			qty, err := s.DynamicQuantityDecrease.GetQuantity(false)
			if err == nil {
				sellQuantity = qty
			} else {
				log.WithError(err).Errorf("cannot get dynamic sell qty of %s, use default qty instead", s.Symbol)
				bbgo.Notify("cannot get dynamic sell qty of %s, use default qty instead", s.Symbol)
			}
		}
	case s.mainTrendCurrent == types.DirectionDown:
		if len(s.DynamicQuantityIncrease) > 0 {
			qty, err := s.DynamicQuantityIncrease.GetQuantity(true)
			if err == nil {
				sellQuantity = qty
			} else {
				log.WithError(err).Errorf("cannot get dynamic sell qty of %s, use default qty instead", s.Symbol)
				bbgo.Notify("cannot get dynamic sell qty of %s, use default qty instead", s.Symbol)
			}
		}
		if len(s.DynamicQuantityDecrease) > 0 {
			qty, err := s.DynamicQuantityDecrease.GetQuantity(true)
			if err == nil {
				buyQuantity = qty
			} else {
				log.WithError(err).Errorf("cannot get dynamic buy qty of %s, use default qty instead", s.Symbol)
				bbgo.Notify("cannot get dynamic buy qty of %s, use default qty instead", s.Symbol)
			}
		}
	}
	if s.UseDynamicQuantityAsAmount {
		log.Infof("caculated %s buy amount %v, sell amount %v", s.Symbol, buyQuantity, sellQuantity)
		qtyAmount := bbgo.QuantityOrAmount{Amount: buyQuantity}
		buyQuantity = qtyAmount.CalculateQuantity(bidPrice)
		qtyAmount.Amount = sellQuantity
		sellQuantity = qtyAmount.CalculateQuantity(askPrice)
		log.Infof("convert %s amount to buy qty %v, sell qty %v", s.Symbol, buyQuantity, sellQuantity)
	} else {
		log.Infof("caculated %s buy qty %v, sell qty %v", s.Symbol, buyQuantity, sellQuantity)
	}

	// Faster position decrease
	if s.mainTrendCurrent == types.DirectionUp && s.SlowLinReg.Last(0) < 0 {
		sellQuantity = sellQuantity.Mul(s.FasterDecreaseRatio)
		log.Infof("faster %s position decrease: sell qty %v", s.Symbol, sellQuantity)
	} else if s.mainTrendCurrent == types.DirectionDown && s.SlowLinReg.Last(0) > 0 {
		buyQuantity = buyQuantity.Mul(s.FasterDecreaseRatio)
		log.Infof("faster %s position decrease: buy qty %v", s.Symbol, buyQuantity)
	}

	// Reduce order qty to fit current position
	if !s.isAllowOppositePosition() {
		if s.Position.IsLong() && s.Position.Base.Abs().Compare(sellQuantity) < 0 {
			sellQuantity = s.Position.Base.Abs()
		} else if s.Position.IsShort() && s.Position.Base.Abs().Compare(buyQuantity) < 0 {
			buyQuantity = s.Position.Base.Abs()
		}
	}

	if buyQuantity.Compare(fixedpoint.Zero) > 0 {
		buyQuantity = s.adjustQuantity(buyQuantity, bidPrice)
	}
	if sellQuantity.Compare(fixedpoint.Zero) > 0 {
		sellQuantity = s.adjustQuantity(sellQuantity, askPrice)
	}

	log.Infof("adjusted sell qty:%v buy qty: %v", sellQuantity, buyQuantity)

	return sellQuantity, buyQuantity
}

// getAllowedBalance returns the allowed qty of orders
func (s *Strategy) getAllowedBalance() (baseQty, quoteQty fixedpoint.Value) {
	// Default
	baseQty = fixedpoint.PosInf
	quoteQty = fixedpoint.PosInf

	balances := s.session.GetAccount().Balances()
	baseBalance, hasBaseBalance := balances[s.Market.BaseCurrency]
	quoteBalance, hasQuoteBalance := balances[s.Market.QuoteCurrency]
	lastPrice, _ := s.session.LastPrice(s.Symbol)

	if bbgo.IsBackTesting { // Backtesting
		baseQty = s.Position.Base
		quoteQty = quoteBalance.Available.Sub(fixedpoint.Max(s.Position.Quote.Mul(fixedpoint.Two), fixedpoint.Zero))
	} else if s.session.Margin || s.session.IsolatedMargin || s.session.Futures || s.session.IsolatedFutures { // Leveraged
		quoteQ, err := bbgo.CalculateQuoteQuantity(s.ctx, s.session, s.Market.QuoteCurrency, s.Leverage)
		if err != nil {
			quoteQ = fixedpoint.Zero
		}
		quoteQty = quoteQ
		baseQty = quoteQ.Div(lastPrice)
	} else { // Spot
		if !hasBaseBalance {
			baseQty = fixedpoint.Zero
		} else {
			baseQty = baseBalance.Available
		}
		if !hasQuoteBalance {
			quoteQty = fixedpoint.Zero
		} else {
			quoteQty = quoteBalance.Available
		}
	}

	return baseQty, quoteQty
}

// getCanBuySell returns the buy sell switches
func (s *Strategy) getCanBuySell(buyQuantity, bidPrice, sellQuantity, askPrice, midPrice fixedpoint.Value) (canBuy bool, canSell bool) {
	// By default, both buy and sell are on, which means we will place buy and sell orders
	canBuy = true
	canSell = true

	// Check if current position > maxExposurePosition
	if s.Position.GetBase().Abs().Compare(s.MaxExposurePosition) > 0 {
		if s.Position.IsLong() {
			canBuy = false
		} else if s.Position.IsShort() {
			canSell = false
		}
		log.Infof("current position %v larger than max exposure %v, skip increase position", s.Position.GetBase().Abs(), s.MaxExposurePosition)
	}

	// Check TradeInBand
	if s.TradeInBand {
		// Price too high
		if bidPrice.Float64() > s.neutralBoll.UpBand.Last(0) {
			canBuy = false
			log.Infof("tradeInBand is set, skip buy due to the price is higher than the neutralBB")
		}
		// Price too low in uptrend
		if askPrice.Float64() < s.neutralBoll.DownBand.Last(0) {
			canSell = false
			log.Infof("tradeInBand is set, skip sell due to the price is lower than the neutralBB")
		}
	}

	// Stop decrease when position closed unless both LinRegs are in the opposite direction to the main trend
	if !s.isAllowOppositePosition() {
		if s.mainTrendCurrent == types.DirectionUp && (s.Position.IsClosed() || s.Position.IsDust(askPrice)) {
			canSell = false
			log.Infof("skip sell due to the long position is closed")
		} else if s.mainTrendCurrent == types.DirectionDown && (s.Position.IsClosed() || s.Position.IsDust(bidPrice)) {
			canBuy = false
			log.Infof("skip buy due to the short position is closed")
		}
	}

	// Min profit
	roi := s.Position.ROI(midPrice)
	if roi.Compare(s.MinProfitActivationRate) >= 0 {
		if s.Position.IsLong() && !s.Position.IsDust(askPrice) {
			minProfitPrice := s.Position.AverageCost.Mul(fixedpoint.One.Add(s.MinProfitSpread))
			if askPrice.Compare(minProfitPrice) < 0 {
				canSell = false
				log.Infof("askPrice %v is less than minProfitPrice %v. skip sell", askPrice, minProfitPrice)
			}
		} else if s.Position.IsShort() && s.Position.IsDust(bidPrice) {
			minProfitPrice := s.Position.AverageCost.Mul(fixedpoint.One.Sub(s.MinProfitSpread))
			if bidPrice.Compare(minProfitPrice) > 0 {
				canBuy = false
				log.Infof("bidPrice %v is greater than minProfitPrice %v. skip buy", bidPrice, minProfitPrice)
			}
		}
	} else {
		log.Infof("position RoI %v is less than minProfitActivationRate %v. min profit protection is not active", roi, s.MinProfitActivationRate)
	}

	// Check against account balance
	baseQty, quoteQty := s.getAllowedBalance()
	if s.session.Margin || s.session.IsolatedMargin || s.session.Futures || s.session.IsolatedFutures { // Leveraged
		if quoteQty.Compare(fixedpoint.Zero) <= 0 {
			if s.Position.IsLong() {
				canBuy = false
				log.Infof("skip buy due to the account has no available balance")
			} else if s.Position.IsShort() {
				canSell = false
				log.Infof("skip sell due to the account has no available balance")
			}
		}
	} else {
		if buyQuantity.Compare(quoteQty.Div(bidPrice)) > 0 { // Spot
			canBuy = false
			log.Infof("skip buy due to the account has no available balance")
		}
		if sellQuantity.Compare(baseQty) > 0 {
			canSell = false
			log.Infof("skip sell due to the account has no available balance")
		}
	}

	log.Infof("canBuy %t, canSell %t", canBuy, canSell)
	return canBuy, canSell
}

// getOrderForms returns buy and sell order form for submission
func (s *Strategy) getOrderForms(buyQuantity, bidPrice, sellQuantity, askPrice fixedpoint.Value) (buyOrder types.SubmitOrder, sellOrder types.SubmitOrder) {
	sellOrder = types.SubmitOrder{
		Symbol:   s.Symbol,
		Side:     types.SideTypeSell,
		Type:     types.OrderTypeLimitMaker,
		Quantity: sellQuantity,
		Price:    askPrice,
		Market:   s.Market,
		GroupID:  s.groupID,
	}
	buyOrder = types.SubmitOrder{
		Symbol:   s.Symbol,
		Side:     types.SideTypeBuy,
		Type:     types.OrderTypeLimitMaker,
		Quantity: buyQuantity,
		Price:    bidPrice,
		Market:   s.Market,
		GroupID:  s.groupID,
	}

	isMargin := s.session.Margin || s.session.IsolatedMargin
	isFutures := s.session.Futures || s.session.IsolatedFutures

	if s.Position.IsClosed() {
		if isMargin {
			buyOrder.MarginSideEffect = types.SideEffectTypeMarginBuy
			sellOrder.MarginSideEffect = types.SideEffectTypeMarginBuy
		} else if isFutures {
			buyOrder.ReduceOnly = false
			sellOrder.ReduceOnly = false
		}
	} else if s.Position.IsLong() {
		if isMargin {
			buyOrder.MarginSideEffect = types.SideEffectTypeMarginBuy
			sellOrder.MarginSideEffect = types.SideEffectTypeAutoRepay
		} else if isFutures {
			buyOrder.ReduceOnly = false
			sellOrder.ReduceOnly = true
		}

		if s.Position.Base.Abs().Compare(sellOrder.Quantity) < 0 {
			if isMargin {
				sellOrder.MarginSideEffect = types.SideEffectTypeMarginBuy
			} else if isFutures {
				sellOrder.ReduceOnly = false
			}
		}
	} else if s.Position.IsShort() {
		if isMargin {
			buyOrder.MarginSideEffect = types.SideEffectTypeAutoRepay
			sellOrder.MarginSideEffect = types.SideEffectTypeMarginBuy
		} else if isFutures {
			buyOrder.ReduceOnly = true
			sellOrder.ReduceOnly = false
		}

		if s.Position.Base.Abs().Compare(buyOrder.Quantity) < 0 {
			if isMargin {
				sellOrder.MarginSideEffect = types.SideEffectTypeMarginBuy
			} else if isFutures {
				sellOrder.ReduceOnly = false
			}
		}
	}

	return buyOrder, sellOrder
}

func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	log.Debugf("%v", orderExecutor) // Here just to suppress GoLand warning
	// initial required information
	s.session = session
	s.ctx = ctx

	// Calculate group id for orders
	instanceID := s.InstanceID()
	s.groupID = util.FNV32(instanceID)

	// If position is nil, we need to allocate a new position for calculation
	if s.Position == nil {
		s.Position = types.NewPositionFromMarket(s.Market)
	}

	// Set fee rate
	if s.session.MakerFeeRate.Sign() > 0 || s.session.TakerFeeRate.Sign() > 0 {
		s.Position.SetExchangeFeeRate(s.session.ExchangeName, types.ExchangeFee{
			MakerFeeRate: s.session.MakerFeeRate,
			TakerFeeRate: s.session.TakerFeeRate,
		})
	}

	// If position is nil, we need to allocate a new position for calculation
	if s.Position == nil {
		s.Position = types.NewPositionFromMarket(s.Market)
	}
	// Always update the position fields
	s.Position.Strategy = ID
	s.Position.StrategyInstanceID = s.InstanceID()

	// Profit stats
	if s.ProfitStats == nil {
		s.ProfitStats = types.NewProfitStats(s.Market)
	}

	if s.TradeStats == nil {
		s.TradeStats = types.NewTradeStats(s.Symbol)
	}

	s.orderExecutor = bbgo.NewGeneralOrderExecutor(session, s.Symbol, ID, instanceID, s.Position)
	s.orderExecutor.BindEnvironment(s.Environment)
	s.orderExecutor.BindProfitStats(s.ProfitStats)
	s.orderExecutor.BindTradeStats(s.TradeStats)
	s.orderExecutor.Bind()
	s.orderExecutor.TradeCollector().OnPositionUpdate(func(position *types.Position) {
		bbgo.Sync(ctx, s)
	})
	s.ExitMethods.Bind(session, s.orderExecutor)

	// Setup profit tracker
	if s.ProfitStatsTracker != nil {
		if s.ProfitStatsTracker.CurrentProfitStats == nil {
			s.ProfitStatsTracker.InitLegacy(s.Market, &s.ProfitStats, s.TradeStats)
		}

		// Add strategy parameters to report
		if s.TrackParameters && s.ProfitStatsTracker.AccumulatedProfitReport != nil {
			s.ProfitStatsTracker.AccumulatedProfitReport.AddStrategyParameter("ReverseEMAWindow", strconv.Itoa(s.ReverseEMA.Window))
			s.ProfitStatsTracker.AccumulatedProfitReport.AddStrategyParameter("FastLinRegWindow", strconv.Itoa(s.FastLinReg.Window))
			s.ProfitStatsTracker.AccumulatedProfitReport.AddStrategyParameter("FastLinRegInterval", s.FastLinReg.Interval.String())
			s.ProfitStatsTracker.AccumulatedProfitReport.AddStrategyParameter("SlowLinRegWindow", strconv.Itoa(s.SlowLinReg.Window))
			s.ProfitStatsTracker.AccumulatedProfitReport.AddStrategyParameter("SlowLinRegInterval", s.SlowLinReg.Interval.String())
			s.ProfitStatsTracker.AccumulatedProfitReport.AddStrategyParameter("FasterDecreaseRatio", strconv.FormatFloat(s.FasterDecreaseRatio.Float64(), 'f', 4, 64))
			s.ProfitStatsTracker.AccumulatedProfitReport.AddStrategyParameter("NeutralBollingerWindow", strconv.Itoa(s.NeutralBollinger.Window))
			s.ProfitStatsTracker.AccumulatedProfitReport.AddStrategyParameter("NeutralBollingerBandWidth", strconv.FormatFloat(s.NeutralBollinger.BandWidth, 'f', 4, 64))
			s.ProfitStatsTracker.AccumulatedProfitReport.AddStrategyParameter("Spread", strconv.FormatFloat(s.Spread.Float64(), 'f', 4, 64))
		}

		s.ProfitStatsTracker.Bind(s.session, s.orderExecutor.TradeCollector())
	}

	// Indicators initialized by StandardIndicatorSet must be initialized in Run()
	// Initialize ReverseEMA
	s.ReverseEMA = s.StandardIndicatorSet.EWMA(s.ReverseEMA.IntervalWindow)
	// Initialize BBs
	s.neutralBoll = s.StandardIndicatorSet.BOLL(s.NeutralBollinger.IntervalWindow, s.NeutralBollinger.BandWidth)

	// Default spread
	if s.Spread == fixedpoint.Zero {
		s.Spread = fixedpoint.NewFromFloat(0.001)
	}

	// StrategyController
	s.Status = types.StrategyStatusRunning
	s.OnSuspend(func() {
		_ = s.orderExecutor.GracefulCancel(ctx)
		bbgo.Sync(ctx, s)
	})
	s.OnEmergencyStop(func() {
		// Close whole position
		_ = s.ClosePosition(ctx, fixedpoint.NewFromFloat(1.0))
	})

	// Initial trend
	session.UserDataStream.OnStart(func() {
		var closePrice fixedpoint.Value
		if !bbgo.IsBackTesting {
			ticker, err := s.session.Exchange.QueryTicker(ctx, s.Symbol)
			if err != nil {
				return
			}

			closePrice = ticker.Buy.Add(ticker.Sell).Div(two)
		} else {
			if price, ok := session.LastPrice(s.Symbol); ok {
				closePrice = price
			}
		}
		priceReverseEMA := fixedpoint.NewFromFloat(s.ReverseEMA.Last(0))

		// Main trend by ReverseEMA
		if closePrice.Compare(priceReverseEMA) > 0 {
			s.mainTrendCurrent = types.DirectionUp
		} else if closePrice.Compare(priceReverseEMA) < 0 {
			s.mainTrendCurrent = types.DirectionDown
		}
	})

	// Check trend reversal
	session.MarketDataStream.OnKLineClosed(types.KLineWith(s.Symbol, s.ReverseInterval, func(kline types.KLine) {
		// closePrice is the close price of current kline
		closePrice := kline.GetClose()
		// priceReverseEMA is the current ReverseEMA price
		priceReverseEMA := fixedpoint.NewFromFloat(s.ReverseEMA.Last(0))

		// Main trend by ReverseEMA
		s.mainTrendPrevious = s.mainTrendCurrent
		if closePrice.Compare(priceReverseEMA) > 0 {
			s.mainTrendCurrent = types.DirectionUp
		} else if closePrice.Compare(priceReverseEMA) < 0 {
			s.mainTrendCurrent = types.DirectionDown
		}
		log.Infof("%s current trend is %v", s.Symbol, s.mainTrendCurrent)

		// Trend reversal
		if s.mainTrendCurrent != s.mainTrendPrevious {
			log.Infof("%s trend reverse to %v", s.Symbol, s.mainTrendCurrent)
			bbgo.Notify("%s trend reverse to %v", s.Symbol, s.mainTrendCurrent)
			// Close on-hand position that is not in the same direction as the new trend
			if !s.Position.IsDust(closePrice) &&
				((s.Position.IsLong() && s.mainTrendCurrent == types.DirectionDown) ||
					(s.Position.IsShort() && s.mainTrendCurrent == types.DirectionUp)) {
				log.Infof("%s closing on-hand position due to trend reverse", s.Symbol)
				bbgo.Notify("%s closing on-hand position due to trend reverse", s.Symbol)
				if err := s.ClosePosition(ctx, fixedpoint.One); err != nil {
					log.WithError(err).Errorf("cannot close on-hand position of %s", s.Symbol)
					bbgo.Notify("cannot close on-hand position of %s", s.Symbol)
				}
			}
		}
	}))

	// Main interval
	session.MarketDataStream.OnKLineClosed(types.KLineWith(s.Symbol, s.Interval, func(kline types.KLine) {
		// StrategyController
		if s.Status != types.StrategyStatusRunning {
			return
		}

		_ = s.orderExecutor.GracefulCancel(ctx)

		// closePrice is the close price of current kline
		closePrice := kline.GetClose()

		// midPrice for ask and bid prices
		var midPrice fixedpoint.Value
		if !bbgo.IsBackTesting {
			ticker, err := s.session.Exchange.QueryTicker(ctx, s.Symbol)
			if err != nil {
				return
			}

			midPrice = ticker.Buy.Add(ticker.Sell).Div(two)
			log.Infof("using ticker price: bid %v / ask %v, mid price %v", ticker.Buy, ticker.Sell, midPrice)
		} else {
			midPrice = closePrice
		}

		// Update price spread
		s.updateSpread()

		// Update max exposure
		s.updateMaxExposure(midPrice)

		// Current position status
		log.Infof("position: %s", s.Position)
		if !s.Position.IsClosed() && !s.Position.IsDust(midPrice) {
			log.Infof("current %s unrealized profit: %f %s", s.Symbol, s.Position.UnrealizedProfit(midPrice).Float64(), s.Market.QuoteCurrency)
		}

		// Order prices
		askPrice, bidPrice := s.getOrderPrices(midPrice)

		// Order qty
		sellQuantity, buyQuantity := s.getOrderQuantities(askPrice, bidPrice)

		buyOrder, sellOrder := s.getOrderForms(buyQuantity, bidPrice, sellQuantity, askPrice)

		canBuy, canSell := s.getCanBuySell(buyQuantity, bidPrice, sellQuantity, askPrice, midPrice)

		// Submit orders
		var submitOrders []types.SubmitOrder
		if canSell && sellOrder.Quantity.Compare(fixedpoint.Zero) > 0 {
			submitOrders = append(submitOrders, sellOrder)
		}
		if canBuy && buyOrder.Quantity.Compare(fixedpoint.Zero) > 0 {
			submitOrders = append(submitOrders, buyOrder)
		}

		if len(submitOrders) == 0 {
			return
		}
		log.Infof("submitting order(s): %v", submitOrders)
		_, _ = s.orderExecutor.SubmitOrders(ctx, submitOrders...)
	}))

	bbgo.OnShutdown(ctx, func(ctx context.Context, wg *sync.WaitGroup) {
		defer wg.Done()

		// Output profit report
		if s.ProfitStatsTracker != nil {
			if s.ProfitStatsTracker.AccumulatedProfitReport != nil {
				s.ProfitStatsTracker.AccumulatedProfitReport.Output()
			}
		}

		_ = s.orderExecutor.GracefulCancel(ctx)
		_, _ = fmt.Fprintln(os.Stderr, s.TradeStats.String())
	})

	return nil
}
