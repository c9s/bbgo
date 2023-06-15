package fixedmaker

import (
	"context"
	"fmt"
	"sync"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/types"
)

const ID = "fixedmaker"

var log = logrus.WithField("strategy", ID)

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

// Fixed spread market making strategy
type Strategy struct {
	Environment          *bbgo.Environment
	StandardIndicatorSet *bbgo.StandardIndicatorSet
	Market               types.Market

	Interval        types.Interval   `json:"interval"`
	Symbol          string           `json:"symbol"`
	Quantity        fixedpoint.Value `json:"quantity"`
	HalfSpreadRatio fixedpoint.Value `json:"halfSpreadRatio"`
	OrderType       types.OrderType  `json:"orderType"`
	DryRun          bool             `json:"dryRun"`

	// SkewFactor is used to calculate the skew of bid/ask price
	SkewFactor   fixedpoint.Value `json:"skewFactor"`
	TargetWeight fixedpoint.Value `json:"targetWeight"`

	// replace halfSpreadRatio by ATR
	ATRMultiplier fixedpoint.Value `json:"atrMultiplier"`
	ATRWindow     int              `json:"atrWindow"`

	// persistence fields
	Position    *types.Position    `json:"position,omitempty" persistence:"position"`
	ProfitStats *types.ProfitStats `json:"profitStats,omitempty" persistence:"profit_stats"`

	session         *bbgo.ExchangeSession
	orderExecutor   *bbgo.GeneralOrderExecutor
	activeOrderBook *bbgo.ActiveOrderBook
	atr             *indicator.ATR
}

func (s *Strategy) Defaults() error {
	if s.OrderType == "" {
		s.OrderType = types.OrderTypeLimitMaker
	}

	if s.ATRWindow == 0 {
		s.ATRWindow = 14
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
	return nil
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.Interval})
}

func (s *Strategy) Run(ctx context.Context, _ bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	s.session = session

	s.activeOrderBook = bbgo.NewActiveOrderBook(s.Symbol)
	s.activeOrderBook.BindStream(session.UserDataStream)

	instanceID := s.InstanceID()

	if s.Position == nil {
		s.Position = types.NewPositionFromMarket(s.Market)
	}

	// Always update the position fields
	s.Position.Strategy = ID
	s.Position.StrategyInstanceID = instanceID

	if s.ProfitStats == nil {
		s.ProfitStats = types.NewProfitStats(s.Market)
	}

	s.orderExecutor = bbgo.NewGeneralOrderExecutor(session, s.Symbol, ID, instanceID, s.Position)
	s.orderExecutor.BindEnvironment(s.Environment)

	s.orderExecutor.BindProfitStats(s.ProfitStats)

	s.orderExecutor.Bind()
	s.orderExecutor.TradeCollector().OnPositionUpdate(func(position *types.Position) {
		bbgo.Sync(ctx, s)
	})

	s.atr = s.StandardIndicatorSet.ATR(types.IntervalWindow{Interval: s.Interval, Window: s.ATRWindow})

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
		_ = s.orderExecutor.GracefulCancel(ctx)
	})

	return nil
}

func (s *Strategy) cancelOrders(ctx context.Context) {
	if err := s.session.Exchange.CancelOrders(ctx, s.activeOrderBook.Orders()...); err != nil {
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

	createdOrders, err := s.orderExecutor.SubmitOrders(ctx, submitOrders...)
	if err != nil {
		log.WithError(err).Error("failed to submit orders")
		return
	}
	log.Infof("created orders: %+v", createdOrders)

	s.activeOrderBook.Add(createdOrders...)
}

func (s *Strategy) generateSubmitOrders(ctx context.Context) ([]types.SubmitOrder, error) {
	orders := []types.SubmitOrder{}

	baseBalance, ok := s.session.GetAccount().Balance(s.Market.BaseCurrency)
	if !ok {
		return nil, fmt.Errorf("base currency %s balance not found", s.Market.BaseCurrency)
	}
	log.Infof("base balance: %+v", baseBalance)

	quoteBalance, ok := s.session.GetAccount().Balance(s.Market.QuoteCurrency)
	if !ok {
		return nil, fmt.Errorf("quote currency %s balance not found", s.Market.QuoteCurrency)
	}
	log.Infof("quote balance: %+v", quoteBalance)

	ticker, err := s.session.Exchange.QueryTicker(ctx, s.Symbol)
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
	amount := s.Quantity.Mul(bidPrice)
	if quoteBalance.Available.Compare(amount) > 0 {
		orders = append(orders, types.SubmitOrder{
			Symbol:   s.Symbol,
			Side:     types.SideTypeBuy,
			Type:     s.OrderType,
			Price:    bidPrice,
			Quantity: s.Quantity,
		})
	} else {
		log.Infof("not enough quote balance to buy, available: %s, amount: %s", quoteBalance.Available, amount)
	}

	if baseBalance.Available.Compare(s.Quantity) > 0 {
		orders = append(orders, types.SubmitOrder{
			Symbol:   s.Symbol,
			Side:     types.SideTypeSell,
			Type:     s.OrderType,
			Price:    askPrice,
			Quantity: s.Quantity,
		})
	} else {
		log.Infof("not enough base balance to sell, available: %s, quantity: %s", baseBalance.Available, s.Quantity)
	}

	return orders, nil
}
