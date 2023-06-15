package rsmaker

import (
	"context"
	"fmt"
	"math"

	"github.com/c9s/bbgo/pkg/indicator"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/muesli/clusters"
	"github.com/muesli/kmeans"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

const ID = "rsmaker"

var notionModifier = fixedpoint.NewFromFloat(1.1)

var log = logrus.WithField("strategy", ID)

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

type Strategy struct {
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
	NeutralBollinger *types.BollingerSetting `json:"neutralBollinger"`

	// DefaultBollinger is the wide range of the bollinger band
	// for controlling your exposure position
	DefaultBollinger *types.BollingerSetting `json:"defaultBollinger"`

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

	Position    *types.Position    `persistence:"position"`
	ProfitStats *types.ProfitStats `persistence:"profit_stats"`
	TradeStats  *types.TradeStats  `persistence:"trade_stats"`

	session       *bbgo.ExchangeSession
	orderExecutor *bbgo.GeneralOrderExecutor
	book          *types.StreamOrderBook

	groupID uint32

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

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{
		Interval: s.Interval,
	})
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

// StrategyController
func (s *Strategy) GetStatus() types.StrategyStatus {
	return s.status
}

func (s *Strategy) Suspend(ctx context.Context) error {
	s.status = types.StrategyStatusStopped

	if err := s.orderExecutor.GracefulCancel(ctx); err != nil {
		log.WithError(err).Errorf("graceful cancel order error")
	}

	bbgo.Sync(ctx, s)
	return nil
}

func (s *Strategy) Resume(ctx context.Context) error {
	s.status = types.StrategyStatusRunning

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

func (s *Strategy) placeOrders(ctx context.Context, midPrice fixedpoint.Value, klines []*types.KLine) {
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

	// logrus.Info(max, min)
	// set up a random two-dimensional data set (float64 values between 0.0 and 1.0)
	var d clusters.Observations
	for x := 0; x < 50; x++ {
		// if klines[x].High.Float64() < max || klines[x].Low.Float64() > min {
		if klines[x].Volume.Float64() > mv*0.3 {
			d = append(d, clusters.Coordinates{
				klines[x].High.Float64(),
				klines[x].Low.Float64(),
				// klines[x].Open.Float64(),
				// klines[x].Close.Float64(),
				// klines[x].Volume.Float64(),
			})
		}
		// }

	}
	log.Info(len(d))

	// Partition the data points into 2 clusters
	km := kmeans.New()
	clusters, err := km.Partition(d, 3)

	// for _, c := range clusters {
	// fmt.Printf("Centered at x: %.2f y: %.2f\n", c.Center[0], c.Center[1])
	// fmt.Printf("Matching data points: %+v\n\n", c.Observations)
	// }
	// clustered virtual kline_1's mid price
	// vk1mp := fixedpoint.NewFromFloat((clusters[0].Center[0] + clusters[0].Center[1]) / 2.)
	// clustered virtual kline_2's mid price
	// vk2mp := fixedpoint.NewFromFloat((clusters[1].Center[0] + clusters[1].Center[1]) / 2.)
	// clustered virtual kline_3's mid price
	// vk3mp := fixedpoint.NewFromFloat((clusters[2].Center[0] + clusters[2].Center[1]) / 2.)

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

	askPrice := fixedpoint.NewFromFloat(math.Max(math.Max(vk1hp.Float64(), vk2hp.Float64()), vk3hp.Float64())) // fixedpoint.NewFromFloat(math.Max(math.Max(vk1mp.Float64(), vk2mp.Float64()), vk3mp.Float64()))
	bidPrice := fixedpoint.NewFromFloat(math.Min(math.Min(vk1lp.Float64(), vk2lp.Float64()), vk3lp.Float64())) // fixedpoint.NewFromFloat(math.Min(math.Min(vk1mp.Float64(), vk2mp.Float64()), vk3mp.Float64()))

	// if vk1mp.Compare(vk2mp) > 0 {
	//	askPrice = vk1mp //.Mul(fixedpoint.NewFromFloat(1.001))
	//	bidPrice = vk2mp //.Mul(fixedpoint.NewFromFloat(0.999))
	// } else if vk1mp.Compare(vk2mp) < 0 {
	//	askPrice = vk2mp //.Mul(fixedpoint.NewFromFloat(1.001))
	//	bidPrice = vk1mp //.Mul(fixedpoint.NewFromFloat(0.999))
	// }
	// midPrice.Mul(fixedpoint.One.Add(askSpread))
	// midPrice.Mul(fixedpoint.One.Sub(bidSpread))
	base := s.Position.GetBase()
	// balances := s.session.GetAccount().Balances()

	canSell := true
	canBuy := true

	// predMidPrice := (askPrice + bidPrice) / 2.

	// if midPrice.Float64() > predMidPrice.Float64() {
	//	bidPrice = predMidPrice.Mul(fixedpoint.NewFromFloat(0.999))
	// }
	//
	// if midPrice.Float64() < predMidPrice.Float64() {
	//	askPrice = predMidPrice.Mul(fixedpoint.NewFromFloat(1.001))
	// }
	//
	// if midPrice.Float64() > askPrice.Float64() {
	//	canBuy = false
	//	askPrice = midPrice.Mul(fixedpoint.NewFromFloat(1.001))
	// }
	//
	// if midPrice.Float64() < bidPrice.Float64() {
	//	canSell = false
	//	bidPrice = midPrice.Mul(fixedpoint.NewFromFloat(0.999))
	// }

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

	// baseBalance, hasBaseBalance := balances[s.Market.BaseCurrency]
	// quoteBalance, hasQuoteBalance := balances[s.Market.QuoteCurrency]

	downBand := s.defaultBoll.DownBand.Last(0)
	upBand := s.defaultBoll.UpBand.Last(0)
	sma := s.defaultBoll.SMA.Last(0)
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

	if canSell {
		submitSellOrders = append(submitSellOrders, sellOrder)
	}
	if canBuy {
		submitBuyOrders = append(submitBuyOrders, buyOrder)
	}

	for i := range submitBuyOrders {
		submitBuyOrders[i] = s.adjustOrderQuantity(submitBuyOrders[i])
	}

	for i := range submitSellOrders {
		submitSellOrders[i] = s.adjustOrderQuantity(submitSellOrders[i])
	}

	if _, err := s.orderExecutor.SubmitOrders(ctx, submitBuyOrders...); err != nil {
		log.WithError(err).Errorf("can not place orders")
	}
	if _, err := s.orderExecutor.SubmitOrders(ctx, submitSellOrders...); err != nil {
		log.WithError(err).Errorf("can not place orders")
	}
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

func (s *Strategy) Run(ctx context.Context, _ bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	instanceID := fmt.Sprintf("%s-%s", ID, s.Symbol)

	s.status = types.StrategyStatusRunning

	if s.Position == nil {
		s.Position = types.NewPositionFromMarket(s.Market)
	}

	if s.ProfitStats == nil {
		s.ProfitStats = types.NewProfitStats(s.Market)
	}

	if s.TradeStats == nil {
		s.TradeStats = types.NewTradeStats(s.Symbol)
	}

	// initial required information
	s.session = session
	s.orderExecutor = bbgo.NewGeneralOrderExecutor(session, s.Symbol, ID, instanceID, s.Position)
	s.orderExecutor.BindEnvironment(s.Environment)
	s.orderExecutor.BindProfitStats(s.ProfitStats)
	s.orderExecutor.BindTradeStats(s.TradeStats)
	s.orderExecutor.TradeCollector().OnPositionUpdate(func(position *types.Position) {
		bbgo.Sync(ctx, s)
	})
	s.orderExecutor.Bind()

	s.neutralBoll = s.StandardIndicatorSet.BOLL(s.NeutralBollinger.IntervalWindow, s.NeutralBollinger.BandWidth)
	s.defaultBoll = s.StandardIndicatorSet.BOLL(s.DefaultBollinger.IntervalWindow, s.DefaultBollinger.BandWidth)

	var klines []*types.KLine
	session.MarketDataStream.OnKLineClosed(func(kline types.KLine) {
		// StrategyController
		if s.status != types.StrategyStatusRunning {
			return
		}

		// if kline.Symbol != s.Symbol || kline.Interval != s.Interval {
		//	return
		// }

		if kline.Interval == s.Interval {
			klines = append(klines, &kline)
		}

		if len(klines) > 50 {
			if kline.Interval == s.Interval {
				if err := s.orderExecutor.GracefulCancel(ctx); err != nil {
					log.WithError(err).Errorf("graceful cancel order error")
				}

				s.placeOrders(ctx, kline.Close, klines[len(klines)-50:])
			}
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
