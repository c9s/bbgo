package linregmaker

import (
	"context"
	"fmt"
	"github.com/c9s/bbgo/pkg/dynamicmetric"
	"sync"

	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/util"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

const ID = "linregmaker"

var notionModifier = fixedpoint.NewFromFloat(1.1)
var two = fixedpoint.NewFromInt(2)

var log = logrus.WithField("strategy", ID)

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
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

	// ReverseEMA is used to determine the long-term trend.
	// Above the ReverseEMA is the long trend and vise versa.
	// All the opposite trend position will be closed upon the trend change
	ReverseEMA *indicator.EWMA `json:"reverseEMA"`

	// mainTrendCurrent is the current long-term trend
	mainTrendCurrent types.Direction
	// mainTrendPrevious is the long-term trend of previous kline
	mainTrendPrevious types.Direction

	// FastLinReg is to determine the short-term trend.
	// Buy/sell orders are placed if the FastLinReg and the ReverseEMA trend are in the same direction, and only orders
	// that reduce position are placed if the FastLinReg and the ReverseEMA trend are in different directions.
	FastLinReg *indicator.LinReg `json:"fastLinReg,omitempty"`

	// SlowLinReg is to determine the midterm trend.
	// When the SlowLinReg and the ReverseEMA trend are in different directions, creation of opposite position is
	// allowed.
	SlowLinReg *indicator.LinReg `json:"slowLinReg,omitempty"`

	// AllowOppositePosition if true, the creation of opposite position is allowed when both fast and slow LinReg are in
	// the opposite direction to main trend
	AllowOppositePosition bool `json:"allowOppositePosition"`

	// FasterDecreaseRatio the quantity of decreasing position orders are multiplied by this ratio when both fast and
	// slow LinReg are in the opposite direction to main trend
	FasterDecreaseRatio fixedpoint.Value `json:"fasterDecreaseRatio,omitempty"`

	// NeutralBollinger is the smaller range of the bollinger band
	// If price is in this band, it usually means the price is oscillating.
	// If price goes out of this band, we tend to not place sell orders or buy orders
	NeutralBollinger *BollingerSetting `json:"neutralBollinger"`

	// TradeInBand
	// When this is on, places orders only when the current price is in the bollinger band.
	TradeInBand bool `json:"tradeInBand"`

	// useTickerPrice use the ticker api to get the mid price instead of the closed kline price.
	// The back-test engine is kline-based, so the ticker price api is not supported.
	// Turn this on if you want to do real trading.
	useTickerPrice bool

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
	DynamicSpread dynamicmetric.DynamicSpread `json:"dynamicSpread,omitempty"`

	// MaxExposurePosition is the maximum position you can hold
	// 10 means you can hold 10 ETH long/short position by maximum
	MaxExposurePosition fixedpoint.Value `json:"maxExposurePosition"`

	// DynamicExposure is used to define the exposure position range with the given percentage.
	// When DynamicExposure is set, your MaxExposurePosition will be calculated dynamically
	DynamicExposure dynamicmetric.DynamicExposure `json:"dynamicExposure"`

	bbgo.QuantityOrAmount
	// TODO: Should work w/o dynamic qty
	// DynamicQuantityIncrease calculates the increase position order quantity dynamically
	DynamicQuantityIncrease dynamicmetric.DynamicQuantitySet `json:"dynamicQuantityIncrease"`

	// DynamicQuantityDecrease calculates the decrease position order quantity dynamically
	DynamicQuantityDecrease dynamicmetric.DynamicQuantitySet `json:"dynamicQuantityDecrease"`

	session *bbgo.ExchangeSession

	// ExitMethods are various TP/SL methods
	ExitMethods bbgo.ExitMethodSet `json:"exits"`

	// persistence fields
	Position    *types.Position    `persistence:"position"`
	ProfitStats *types.ProfitStats `persistence:"profit_stats"`
	TradeStats  *types.TradeStats  `persistence:"trade_stats"`

	orderExecutor *bbgo.GeneralOrderExecutor

	groupID uint32

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
	// Subscribe for ReverseEMA
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{
		Interval: s.ReverseEMA.Interval,
	})
	// Initialize ReverseEMA
	s.ReverseEMA = s.StandardIndicatorSet.EWMA(s.ReverseEMA.IntervalWindow)

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
	if s.NeutralBollinger != nil && s.NeutralBollinger.Interval != "" {
		session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{
			Interval: s.NeutralBollinger.Interval,
		})
	}
	// Initialize BBs
	s.neutralBoll = s.StandardIndicatorSet.BOLL(s.NeutralBollinger.IntervalWindow, s.NeutralBollinger.BandWidth)

	// Setup Exits
	s.ExitMethods.SetAndSubscribe(session, s)

	// Setup dynamic spread
	if s.DynamicSpread.IsEnabled() {
		s.DynamicSpread.Initialize(s.Symbol, session)
	}

	// Setup dynamic exposure
	if s.DynamicExposure.IsEnabled() {
		s.DynamicExposure.Initialize(s.Symbol, session, s.StandardIndicatorSet)
	}

	// Setup dynamic quantities
	if len(s.DynamicQuantityIncrease) > 0 {
		s.DynamicQuantityIncrease.Initialize(s.Symbol, session)
	}
	if len(s.DynamicQuantityDecrease) > 0 {
		s.DynamicQuantityDecrease.Initialize(s.Symbol, session)
	}
}

// TODO
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
		maxExposurePosition, err := s.DynamicExposure.GetMaxExposure(midPrice.Float64())
		if err != nil {
			log.WithError(err).Errorf("can not calculate DynamicExposure of %s, use previous MaxExposurePosition instead", s.Symbol)
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
	log.Infof("mid price:%v ask:%v bid: %v", midPrice, askPrice, bidPrice)

	return askPrice, bidPrice
}

// getOrderQuantities returns sell and buy qty
func (s *Strategy) getOrderQuantities(askPrice fixedpoint.Value, bidPrice fixedpoint.Value) (sellQuantity fixedpoint.Value, buyQuantity fixedpoint.Value) {
	// TODO: spot, margin, and futures

	// Dynamic qty
	switch {
	case s.mainTrendCurrent == types.DirectionUp:
		var err error
		if len(s.DynamicQuantityIncrease) > 0 {
			buyQuantity, err = s.DynamicQuantityIncrease.GetQuantity()
			if err != nil {
				buyQuantity = s.QuantityOrAmount.CalculateQuantity(bidPrice)
			}
		}
		if len(s.DynamicQuantityDecrease) > 0 {
			sellQuantity, err = s.DynamicQuantityDecrease.GetQuantity()
			if err != nil {
				sellQuantity = s.QuantityOrAmount.CalculateQuantity(askPrice)
			}
		}
	case s.mainTrendCurrent == types.DirectionDown:
		var err error
		if len(s.DynamicQuantityIncrease) > 0 {
			sellQuantity, err = s.DynamicQuantityIncrease.GetQuantity()
			if err != nil {
				sellQuantity = s.QuantityOrAmount.CalculateQuantity(bidPrice)
			}
		}
		if len(s.DynamicQuantityDecrease) > 0 {
			buyQuantity, err = s.DynamicQuantityDecrease.GetQuantity()
			if err != nil {
				buyQuantity = s.QuantityOrAmount.CalculateQuantity(askPrice)
			}
		}
	default:
		sellQuantity = s.QuantityOrAmount.CalculateQuantity(askPrice)
		buyQuantity = s.QuantityOrAmount.CalculateQuantity(bidPrice)
	}

	// Faster position decrease
	if s.mainTrendCurrent == types.DirectionUp && s.FastLinReg.Last() < 0 && s.SlowLinReg.Last() < 0 {
		sellQuantity = sellQuantity * s.FasterDecreaseRatio
	} else if s.mainTrendCurrent == types.DirectionDown && s.FastLinReg.Last() > 0 && s.SlowLinReg.Last() > 0 {
		buyQuantity = buyQuantity * s.FasterDecreaseRatio
	}

	log.Infof("sell qty:%v buy qty: %v", sellQuantity, buyQuantity)

	return sellQuantity, buyQuantity
}

// getCanBuySell returns the buy sell switches
func (s *Strategy) getCanBuySell(midPrice fixedpoint.Value) (canBuy bool, canSell bool) {
	// By default, both buy and sell are on, which means we will place buy and sell orders
	canBuy = true
	canSell = true

	// Check if current position > maxExposurePosition
	if s.Position.GetBase().Abs().Compare(s.MaxExposurePosition) > 0 {
		if s.mainTrendCurrent == types.DirectionUp {
			canBuy = false
		} else if s.mainTrendCurrent == types.DirectionDown {
			canSell = false
		}
	}

	if s.TradeInBand {
		// Price too high
		if midPrice.Float64() > s.neutralBoll.UpBand.Last() {
			canBuy = false
			log.Infof("tradeInBand is set, skip buy when the price is higher than the neutralBB")
		}
		// Price too low in uptrend
		if midPrice.Float64() < s.neutralBoll.DownBand.Last() {
			canSell = false
			log.Infof("tradeInBand is set, skip sell when the price is lower than the neutralBB")
		}
	}

	// Stop decrease when position closed unless both LinRegs are in the opposite direction to the main trend
	if s.Position.IsClosed() || s.Position.IsDust(midPrice) {
		if s.mainTrendCurrent == types.DirectionUp && !(s.AllowOppositePosition && s.FastLinReg.Last() < 0 && s.SlowLinReg.Last() < 0) {
			canSell = false
		} else if s.mainTrendCurrent == types.DirectionDown && !(s.AllowOppositePosition && s.FastLinReg.Last() > 0 && s.SlowLinReg.Last() > 0) {
			canBuy = false
		}
	}

	return canBuy, canSell
}

func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	// initial required information
	s.session = session

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

	if bbgo.IsBackTesting {
		s.useTickerPrice = false
	} else {
		s.useTickerPrice = true
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

	// Main interval
	session.MarketDataStream.OnKLineClosed(types.KLineWith(s.Symbol, s.Interval, func(kline types.KLine) {
		// StrategyController
		if s.Status != types.StrategyStatusRunning {
			return
		}

		_ = s.orderExecutor.GracefulCancel(ctx)

		// closePrice is the close price of current kline
		closePrice := kline.GetClose()
		// priceReverseEMA is the current ReverseEMA price
		priceReverseEMA := fixedpoint.NewFromFloat(s.ReverseEMA.Last())

		// Main trend by ReverseEMA
		s.mainTrendPrevious = s.mainTrendCurrent
		if closePrice.Compare(priceReverseEMA) > 0 {
			s.mainTrendCurrent = types.DirectionUp
		} else if closePrice.Compare(priceReverseEMA) < 0 {
			s.mainTrendCurrent = types.DirectionDown
		}

		// Trend reversal
		if s.mainTrendCurrent != s.mainTrendPrevious {
			// Close on-hand position that is not in the same direction as the new trend
			if !s.Position.IsDust(closePrice) &&
				((s.Position.IsLong() && s.mainTrendCurrent == types.DirectionDown) ||
					(s.Position.IsShort() && s.mainTrendCurrent == types.DirectionUp)) {
				log.Infof("trend reverse to %v. closing on-hand position", s.mainTrendCurrent)
				if err := s.ClosePosition(ctx, fixedpoint.One); err != nil {
					log.WithError(err).Errorf("cannot close on-hand position of %s", s.Symbol)
					// TODO: close position failed. retry?
				}
			}
		}

		// midPrice for ask and bid prices
		var midPrice fixedpoint.Value
		if s.useTickerPrice {
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

		// TODO: Reduce only in margin and futures
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

		canBuy, canSell := s.getCanBuySell(midPrice)

		// TODO: check enough balance?

		// Submit orders
		var submitOrders []types.SubmitOrder
		if canSell {
			submitOrders = append(submitOrders, adjustOrderQuantity(sellOrder, s.Market))
		}
		if canBuy {
			submitOrders = append(submitOrders, adjustOrderQuantity(buyOrder, s.Market))
		}

		if len(submitOrders) == 0 {
			return
		}
		_, _ = s.orderExecutor.SubmitOrders(ctx, submitOrders...)
	}))

	bbgo.OnShutdown(ctx, func(ctx context.Context, wg *sync.WaitGroup) {
		defer wg.Done()

		_ = s.orderExecutor.GracefulCancel(ctx)
	})

	return nil
}

// TODO
func adjustOrderQuantity(submitOrder types.SubmitOrder, market types.Market) types.SubmitOrder {
	if submitOrder.Quantity.Mul(submitOrder.Price).Compare(market.MinNotional) < 0 {
		submitOrder.Quantity = bbgo.AdjustFloatQuantityByMinAmount(submitOrder.Quantity, submitOrder.Price, market.MinNotional.Mul(notionModifier))
	}

	if submitOrder.Quantity.Compare(market.MinQuantity) < 0 {
		submitOrder.Quantity = fixedpoint.Max(submitOrder.Quantity, market.MinQuantity)
	}

	return submitOrder
}
