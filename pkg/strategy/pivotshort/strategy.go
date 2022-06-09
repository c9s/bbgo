package pivotshort

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/types"
)

const ID = "pivotshort"

var log = logrus.WithField("strategy", ID)

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

type IntervalWindowSetting struct {
	types.IntervalWindow
}

// BreakLow -- when price breaks the previous pivot low, we set a trade entry
type BreakLow struct {
	Ratio    fixedpoint.Value `json:"ratio"`
	Quantity fixedpoint.Value `json:"quantity"`
}

type Entry struct {
	CatBounceRatio fixedpoint.Value `json:"catBounceRatio"`
	NumLayers      int              `json:"numLayers"`
	TotalQuantity  fixedpoint.Value `json:"totalQuantity"`

	Quantity         fixedpoint.Value                `json:"quantity"`
	MarginSideEffect types.MarginOrderSideEffectType `json:"marginOrderSideEffect"`
}

type Exit struct {
	RoiStopLossPercentage   fixedpoint.Value `json:"roiStopLossPercentage"`
	RoiTakeProfitPercentage fixedpoint.Value `json:"roiTakeProfitPercentage"`

	LowerShadowRatio fixedpoint.Value `json:"lowerShadowRatio"`

	MarginSideEffect types.MarginOrderSideEffectType `json:"marginOrderSideEffect"`
}

type Strategy struct {
	*bbgo.Graceful
	*bbgo.Notifiability
	*bbgo.Persistence

	Environment *bbgo.Environment
	Symbol      string `json:"symbol"`
	Market      types.Market
	Interval    types.Interval `json:"interval"`

	// persistence fields
	Position    *types.Position    `json:"position,omitempty" persistence:"position"`
	ProfitStats *types.ProfitStats `json:"profitStats,omitempty" persistence:"profit_stats"`

	PivotLength int `json:"pivotLength"`

	BreakLow BreakLow `json:"breakLow"`
	Entry    Entry    `json:"entry"`
	Exit     Exit     `json:"exit"`

	activeMakerOrders *bbgo.ActiveOrderBook
	orderStore        *bbgo.OrderStore
	tradeCollector    *bbgo.TradeCollector

	session *bbgo.ExchangeSession

	lastLow        fixedpoint.Value
	pivot          *indicator.Pivot
	pivotLowPrices []fixedpoint.Value

	// StrategyController
	bbgo.StrategyController
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	log.Infof("subscribe %s", s.Symbol)
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.Interval})
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: types.Interval1m})
}

func (s *Strategy) submitOrders(ctx context.Context, orderExecutor bbgo.OrderExecutor, submitOrders ...types.SubmitOrder) {
	createdOrders, err := orderExecutor.SubmitOrders(ctx, submitOrders...)
	if err != nil {
		log.WithError(err).Errorf("can not place orders")
	}

	s.orderStore.Add(createdOrders...)
	s.activeMakerOrders.Add(createdOrders...)
	s.tradeCollector.Process()
}

func (s *Strategy) placeMarketSell(ctx context.Context, orderExecutor bbgo.OrderExecutor, quantity fixedpoint.Value) {
	if quantity.IsZero() {
		if balance, ok := s.session.Account.Balance(s.Market.BaseCurrency); ok {
			s.Notify("sell quantity is not set, submitting sell with all base balance: %s", balance.Available.String())
			quantity = balance.Available
		}
	}

	if quantity.IsZero() {
		log.Errorf("quantity is zero, can not submit sell order, please check settings")
		return
	}

	submitOrder := types.SubmitOrder{
		Symbol:           s.Symbol,
		Side:             types.SideTypeSell,
		Type:             types.OrderTypeMarket,
		Quantity:         quantity,
		MarginSideEffect: types.SideEffectTypeMarginBuy,
	}

	s.submitOrders(ctx, orderExecutor, submitOrder)
}

func (s *Strategy) ClosePosition(ctx context.Context, percentage fixedpoint.Value) error {
	submitOrder := s.Position.NewMarketCloseOrder(percentage) // types.SubmitOrder{
	if submitOrder == nil {
		return nil
	}

	if s.session.Margin {
		submitOrder.MarginSideEffect = s.Exit.MarginSideEffect
	}

	s.Notify("Closing %s position by %f", s.Symbol, percentage.Float64())

	createdOrders, err := s.session.Exchange.SubmitOrders(ctx, *submitOrder)
	if err != nil {
		log.WithError(err).Errorf("can not place position close order")
	}

	s.orderStore.Add(createdOrders...)
	s.activeMakerOrders.Add(createdOrders...)
	s.tradeCollector.Process()
	return err
}
func (s *Strategy) InstanceID() string {
	return fmt.Sprintf("%s:%s", ID, s.Symbol)
}

func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	// initial required information
	s.session = session

	s.activeMakerOrders = bbgo.NewActiveOrderBook(s.Symbol)
	s.activeMakerOrders.BindStream(session.UserDataStream)

	s.orderStore = bbgo.NewOrderStore(s.Symbol)
	s.orderStore.BindStream(session.UserDataStream)

	if s.Position == nil {
		s.Position = types.NewPositionFromMarket(s.Market)
	}

	if s.ProfitStats == nil {
		s.ProfitStats = types.NewProfitStats(s.Market)
	}

	instanceID := s.InstanceID()

	// Always update the position fields
	s.Position.Strategy = ID
	s.Position.StrategyInstanceID = instanceID

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

	iw := types.IntervalWindow{Window: s.PivotLength, Interval: s.Interval}
	store, _ := session.MarketDataStore(s.Symbol)
	s.pivot = &indicator.Pivot{IntervalWindow: iw}
	s.pivot.Bind(store)

	s.lastLow = fixedpoint.Zero

	session.UserDataStream.OnStart(func() {
		/*
			if price, ok := session.LastPrice(s.Symbol); ok {
				if limitPrice, ok := s.findHigherPivotLow(price); ok {
					log.Infof("%s placing limit sell start from %f adds up to %f percent with %d layers of orders", s.Symbol, limitPrice.Float64(), s.Entry.CatBounceRatio.Mul(fixedpoint.NewFromInt(100)).Float64(), s.Entry.NumLayers)
					s.placeBounceSellOrders(ctx, limitPrice, price, orderExecutor)
				}
			}
		*/
	})

	// Always check whether you can open a short position or not
	session.MarketDataStream.OnKLineClosed(func(kline types.KLine) {
		if kline.Symbol != s.Symbol || kline.Interval != types.Interval1m {
			return
		}

		isPositionOpened := !s.Position.IsClosed() && !s.Position.IsDust(kline.Close)

		if isPositionOpened && s.Position.IsShort() {
			// calculate return rate
			// TODO: apply quantity to this formula
			roi := kline.Close.Sub(s.Position.AverageCost).Div(s.Position.AverageCost)
			if roi.Compare(s.Exit.RoiStopLossPercentage) > 0 {
				// SL
				s.Notify("%s ROI StopLoss triggered at price %f", s.Symbol, kline.Close.Float64())
				if err := s.activeMakerOrders.GracefulCancel(ctx, s.session.Exchange); err != nil {
					log.WithError(err).Errorf("graceful cancel order error")
				}

				if err := s.ClosePosition(ctx, fixedpoint.One); err != nil {
					log.WithError(err).Errorf("close position error")
				}

				return
			} else if roi.Compare(s.Exit.RoiTakeProfitPercentage.Neg()) < 0 {
				s.Notify("%s TakeProfit triggered at price %f", s.Symbol, kline.Close.Float64())
				if err := s.activeMakerOrders.GracefulCancel(ctx, s.session.Exchange); err != nil {
					log.WithError(err).Errorf("graceful cancel order error")
				}

				if err := s.ClosePosition(ctx, fixedpoint.One); err != nil {
					log.WithError(err).Errorf("close position error")
				}
				return
			} else if !s.Exit.LowerShadowRatio.IsZero() && kline.GetLowerShadowHeight().Div(kline.Low).Compare(s.Exit.LowerShadowRatio) > 0 {
				s.Notify("%s TakeProfit triggered at price %f: shadow ratio %f", s.Symbol, kline.Close.Float64(), kline.GetLowerShadowRatio().Float64(), kline)
				if err := s.activeMakerOrders.GracefulCancel(ctx, s.session.Exchange); err != nil {
					log.WithError(err).Errorf("graceful cancel order error")
				}

				if err := s.ClosePosition(ctx, fixedpoint.One); err != nil {
					log.WithError(err).Errorf("close position error")
				}
				return
			}
		}

		if len(s.pivotLowPrices) == 0 {
			return
		}

		previousLow := s.pivotLowPrices[len(s.pivotLowPrices)-1]

		// truncate the pivot low prices
		if len(s.pivotLowPrices) > 10 {
			s.pivotLowPrices = s.pivotLowPrices[len(s.pivotLowPrices)-10:]
		}

		ratio := fixedpoint.One.Sub(s.BreakLow.Ratio)
		breakPrice := previousLow.Mul(ratio)
		if kline.Close.Compare(breakPrice) > 0 {
			return
		}

		if !s.Position.IsClosed() && !s.Position.IsDust(kline.Close) {
			// s.Notify("skip opening %s position, which is not closed", s.Symbol, s.Position)
			return
		}

		s.Notify("%s price %f breaks the previous low %f with ratio %f, submitting market sell to open a short position", s.Symbol, kline.Close.Float64(), previousLow.Float64(), s.BreakLow.Ratio.Float64())

		if err := s.activeMakerOrders.GracefulCancel(ctx, s.session.Exchange); err != nil {
			log.WithError(err).Errorf("graceful cancel order error")
		}

		s.placeMarketSell(ctx, orderExecutor, s.BreakLow.Quantity)
	})

	session.MarketDataStream.OnKLineClosed(func(kline types.KLine) {
		if kline.Symbol != s.Symbol || kline.Interval != s.Interval {
			return
		}

		if s.pivot.LastLow() > 0.0 {
			log.Infof("pivot low detected: %f %s", s.pivot.LastLow(), kline.EndTime.Time())
			lastLow := fixedpoint.NewFromFloat(s.pivot.LastLow())
			if lastLow.Compare(s.lastLow) != 0 {
				s.lastLow = lastLow
				s.pivotLowPrices = append(s.pivotLowPrices, s.lastLow)
			}
		}
	})

	return nil
}

func (s *Strategy) findHigherPivotLow(price fixedpoint.Value) (fixedpoint.Value, bool) {
	for l := len(s.pivotLowPrices) - 1; l > 0; l-- {
		if s.pivotLowPrices[l].Compare(price) > 0 {
			return s.pivotLowPrices[l], true
		}
	}

	return price, false
}

func (s *Strategy) placeBounceSellOrders(ctx context.Context, lastLow fixedpoint.Value, limitPrice fixedpoint.Value, currentPrice fixedpoint.Value, orderExecutor bbgo.OrderExecutor) {
	futuresMode := s.session.Futures || s.session.IsolatedFutures
	numLayers := fixedpoint.NewFromInt(int64(s.Entry.NumLayers))
	d := s.Entry.CatBounceRatio.Div(numLayers)
	q := s.Entry.Quantity
	if !s.Entry.TotalQuantity.IsZero() {
		q = s.Entry.TotalQuantity.Div(numLayers)
	}

	for i := 0; i < s.Entry.NumLayers; i++ {
		balances := s.session.GetAccount().Balances()
		quoteBalance, _ := balances[s.Market.QuoteCurrency]
		baseBalance, _ := balances[s.Market.BaseCurrency]

		p := limitPrice.Mul(fixedpoint.One.Add(s.Entry.CatBounceRatio.Sub(fixedpoint.NewFromFloat(d.Float64() * float64(i)))))

		if futuresMode {
			if q.Mul(p).Compare(quoteBalance.Available) <= 0 {
				s.placeOrder(ctx, lastLow, p, currentPrice, q, orderExecutor)
			}
		} else if s.Environment.IsBackTesting() {
			if q.Compare(baseBalance.Available) <= 0 {
				s.placeOrder(ctx, lastLow, p, currentPrice, q, orderExecutor)
			}
		} else {
			if q.Compare(baseBalance.Available) <= 0 {
				s.placeOrder(ctx, lastLow, p, currentPrice, q, orderExecutor)
			}
		}
	}
}

func (s *Strategy) placeOrder(ctx context.Context, lastLow fixedpoint.Value, limitPrice fixedpoint.Value, currentPrice fixedpoint.Value, qty fixedpoint.Value, orderExecutor bbgo.OrderExecutor) {
	submitOrder := types.SubmitOrder{
		Symbol:   s.Symbol,
		Side:     types.SideTypeSell,
		Type:     types.OrderTypeLimit,
		Price:    limitPrice,
		Quantity: qty,
	}

	if !lastLow.IsZero() && lastLow.Compare(currentPrice) <= 0 {
		submitOrder.Type = types.OrderTypeMarket
	}

	s.submitOrders(ctx, orderExecutor, submitOrder)
}
