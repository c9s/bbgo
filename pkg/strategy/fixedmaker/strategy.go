package fixedmaker

import (
	"context"
	"fmt"
	"sync"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	indicatorv2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/strategy/common"
	"github.com/c9s/bbgo/pkg/types"
)

const ID = "fixedmaker"

var log = logrus.WithField("strategy", ID)

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

// Fixed spread market making strategy
type Strategy struct {
	*common.Strategy

	Environment          *bbgo.Environment
	StandardIndicatorSet *bbgo.StandardIndicatorSet
	Market               types.Market

	Interval        types.Interval   `json:"interval"`
	Symbol          string           `json:"symbol"`
	Quantity        fixedpoint.Value `json:"quantity"`
	HalfSpreadRatio fixedpoint.Value `json:"halfSpreadRatio"`
	OrderType       types.OrderType  `json:"orderType"`

	// SkewFactor is used to calculate the skew of bid/ask price
	SkewFactor   fixedpoint.Value `json:"skewFactor"`
	TargetWeight fixedpoint.Value `json:"targetWeight"`

	// replace halfSpreadRatio by ATR
	ATRMultiplier fixedpoint.Value `json:"atrMultiplier"`
	ATRWindow     int              `json:"atrWindow"`

	// bollinger band is used to check if the price is too high or too low
	BOLLWindow    int     `json:"bollWindow"`
	BOLLBandWidth float64 `json:"bollBandWidth"`

	DryRun bool `json:"dryRun"`

	activeOrderBook *bbgo.ActiveOrderBook
	atr             *indicatorv2.ATRStream
	boll            *indicatorv2.BOLLStream
}

func (s *Strategy) Defaults() error {
	if s.OrderType == "" {
		log.Infof("order type is not set, using limit maker order type")
		s.OrderType = types.OrderTypeLimitMaker
	}

	if s.ATRWindow == 0 {
		log.Infof("atr window is not set, using default value 14")
		s.ATRWindow = 14
	}

	if s.BOLLWindow == 0 {
		log.Infof("boll window is not set, using default value 20")
		s.BOLLWindow = 20
	}

	if s.BOLLBandWidth == 0 {
		log.Infof("boll band width is not set, using default value 2.0")
		s.BOLLBandWidth = 2.0
	}
	return nil
}
func (s *Strategy) Initialize() error {
	return nil
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) InstanceID() string {
	return fmt.Sprintf("%s:%s", ID, s.Symbol)
}

func (s *Strategy) Validate() error {
	if s.Quantity.Float64() <= 0 {
		return fmt.Errorf("quantity should be positive")
	}

	if s.HalfSpreadRatio.Float64() <= 0 {
		return fmt.Errorf("halfSpreadRatio should be positive")
	}

	if s.SkewFactor.Float64() < 0 {
		return fmt.Errorf("skewFactor should be non-negative")
	}

	if s.ATRMultiplier.Float64() < 0 {
		return fmt.Errorf("atrMultiplier should be non-negative")
	}

	if s.ATRWindow < 0 {
		return fmt.Errorf("atrWindow should be non-negative")
	}

	if s.BOLLWindow < 0 {
		return fmt.Errorf("bollWindow should be non-negative")
	}

	if s.BOLLBandWidth < 0 {
		return fmt.Errorf("bollBandWidth should be non-negative")
	}
	return nil
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.Interval})
}

func (s *Strategy) Run(ctx context.Context, _ bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	s.Strategy = &common.Strategy{}
	s.Strategy.Initialize(ctx, s.Environment, session, s.Market, ID, s.InstanceID())

	s.activeOrderBook = bbgo.NewActiveOrderBook(s.Symbol)
	s.activeOrderBook.BindStream(session.UserDataStream)

	s.atr = session.Indicators(s.Symbol).ATR(s.Interval, s.ATRWindow)
	s.boll = session.Indicators(s.Symbol).BOLL(types.IntervalWindow{Interval: s.Interval, Window: s.BOLLWindow}, s.BOLLBandWidth)

	session.UserDataStream.OnStart(func() {
		// you can place orders here when bbgo is started, this will be called only once.
		s.replenish(ctx)
	})

	s.activeOrderBook.OnFilled(func(order types.Order) {
		if s.activeOrderBook.NumOfOrders() == 0 {
			log.Infof("no active orders, replenish")
			s.replenish(ctx)
		}
	})

	session.MarketDataStream.OnKLineClosed(func(kline types.KLine) {
		log.Infof("%+v", kline)

		s.cancelOrders(ctx)
		s.replenish(ctx)
	})

	// the shutdown handler, you can cancel all orders
	bbgo.OnShutdown(ctx, func(ctx context.Context, wg *sync.WaitGroup) {
		defer wg.Done()
		_ = s.OrderExecutor.GracefulCancel(ctx)
	})

	return nil
}

func (s *Strategy) cancelOrders(ctx context.Context) {
	if err := s.Session.Exchange.CancelOrders(ctx, s.activeOrderBook.Orders()...); err != nil {
		log.WithError(err).Errorf("failed to cancel orders")
	}
}

func (s *Strategy) replenish(ctx context.Context) {
	submitOrders, err := s.generateSubmitOrders(ctx)
	if err != nil {
		log.WithError(err).Error("failed to generate submit orders")
		return
	}
	log.Infof("submit orders: %+v", submitOrders)

	if s.DryRun {
		log.Infof("dry run, not submitting orders")
		return
	}

	createdOrders, err := s.OrderExecutor.SubmitOrders(ctx, submitOrders...)
	if err != nil {
		log.WithError(err).Error("failed to submit orders")
		return
	}
	log.Infof("created orders: %+v", createdOrders)

	s.activeOrderBook.Add(createdOrders...)
}

func (s *Strategy) generateSubmitOrders(ctx context.Context) ([]types.SubmitOrder, error) {
	orders := []types.SubmitOrder{}

	baseBalance, ok := s.Session.GetAccount().Balance(s.Market.BaseCurrency)
	if !ok {
		return nil, fmt.Errorf("base currency %s balance not found", s.Market.BaseCurrency)
	}
	log.Infof("base balance: %+v", baseBalance)

	quoteBalance, ok := s.Session.GetAccount().Balance(s.Market.QuoteCurrency)
	if !ok {
		return nil, fmt.Errorf("quote currency %s balance not found", s.Market.QuoteCurrency)
	}
	log.Infof("quote balance: %+v", quoteBalance)

	ticker, err := s.Session.Exchange.QueryTicker(ctx, s.Symbol)
	if err != nil {
		return nil, err
	}
	midPrice := ticker.Buy.Add(ticker.Sell).Div(fixedpoint.NewFromFloat(2.0))
	log.Infof("mid price: %+v", midPrice)

	if s.ATRMultiplier.Float64() > 0 {
		atr := fixedpoint.NewFromFloat(s.atr.Last(0))
		log.Infof("atr: %s", atr.String())
		s.HalfSpreadRatio = s.ATRMultiplier.Mul(atr).Div(midPrice)
		log.Infof("half spread ratio: %s", s.HalfSpreadRatio.String())
	}

	// calcualte skew by the difference between base weight and target weight
	baseValue := baseBalance.Total().Mul(midPrice)
	baseWeight := baseValue.Div(baseValue.Add(quoteBalance.Total()))
	skew := s.SkewFactor.Mul(s.HalfSpreadRatio).Mul(baseWeight.Sub(s.TargetWeight))

	// let the skew be in the range of [-r, r]
	skew = skew.Clamp(s.HalfSpreadRatio.Neg(), s.HalfSpreadRatio)

	// calculate bid and ask price
	// bid price = mid price * (1 - r - skew))
	bidSpreadRatio := fixedpoint.Max(s.HalfSpreadRatio.Add(skew), fixedpoint.Zero)
	bidPrice := midPrice.Mul(fixedpoint.One.Sub(bidSpreadRatio))
	log.Infof("bid price: %s", bidPrice.String())
	// ask price = mid price * (1 + r - skew))
	askSrasedRatio := fixedpoint.Max(s.HalfSpreadRatio.Sub(skew), fixedpoint.Zero)
	askPrice := midPrice.Mul(fixedpoint.One.Add(askSrasedRatio))
	log.Infof("ask price: %s", askPrice.String())

	// check balance and generate orders
	orders = s.appendBuyOrder(orders, bidPrice, quoteBalance)
	orders = s.appendSellOrder(orders, askPrice, baseBalance)

	return orders, nil
}

func (s *Strategy) appendBuyOrder(orders []types.SubmitOrder, bidPrice fixedpoint.Value, quoteBalance types.Balance) []types.SubmitOrder {
	amount := s.Quantity.Mul(bidPrice)
	if quoteBalance.Available.Compare(amount) < 0 {
		log.Infof("not enough quote balance to buy, available: %s, amount: %s", quoteBalance.Available, amount)
		return orders
	}

	if bidPrice.Float64() > s.boll.UpBand.Last(0) {
		log.Infof("bid price %s is higher than boll up band %f, skip buying", bidPrice.String(), s.boll.UpBand.Last(0))
		return orders
	}

	return append(orders, types.SubmitOrder{
		Symbol:   s.Symbol,
		Side:     types.SideTypeBuy,
		Type:     s.OrderType,
		Price:    bidPrice,
		Quantity: s.Quantity,
	})
}

func (s *Strategy) appendSellOrder(orders []types.SubmitOrder, askPrice fixedpoint.Value, baseBalance types.Balance) []types.SubmitOrder {
	if baseBalance.Available.Compare(s.Quantity) < 0 {
		log.Infof("not enough base balance to sell, available: %s, quantity: %s", baseBalance.Available, s.Quantity)
		return orders
	}

	if askPrice.Float64() < s.boll.DownBand.Last(0) {
		log.Infof("ask price %s is lower than boll down band %f, skip selling", askPrice.String(), s.boll.DownBand.Last(0))
		return orders
	}

	return append(orders, types.SubmitOrder{
		Symbol:   s.Symbol,
		Side:     types.SideTypeSell,
		Type:     s.OrderType,
		Price:    askPrice,
		Quantity: s.Quantity,
	})
}
