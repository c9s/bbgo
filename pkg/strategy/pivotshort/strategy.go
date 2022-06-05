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

type Entry struct {
	Immediate        bool                            `json:"immediate"`
	CatBounceRatio   fixedpoint.Value                `json:"catBounceRatio"`
	Quantity         fixedpoint.Value                `json:"quantity"`
	NumLayers        int                             `json:"numLayers"`
	MarginSideEffect types.MarginOrderSideEffectType `json:"marginOrderSideEffect"`
}

type Exit struct {
	TakeProfitPercentage fixedpoint.Value                `json:"takeProfitPercentage"`
	StopLossPercentage   fixedpoint.Value                `json:"stopLossPercentage"`
	ShadowTPRatio        fixedpoint.Value                `json:"shadowTakeProfitRatio"`
	MarginSideEffect     types.MarginOrderSideEffectType `json:"marginOrderSideEffect"`
}

type Strategy struct {
	*bbgo.Graceful
	*bbgo.Notifiability
	*bbgo.Persistence

	Environment   *bbgo.Environment
	Symbol        string `json:"symbol"`
	Market        types.Market
	Interval      types.Interval   `json:"interval"`
	TotalQuantity fixedpoint.Value `json:"totalQuantity"`

	// persistence fields
	Position    *types.Position    `json:"position,omitempty" persistence:"position"`
	ProfitStats *types.ProfitStats `json:"profitStats,omitempty" persistence:"profit_stats"`

	PivotLength int `json:"pivotLength"`
	LastLow     fixedpoint.Value

	Entry Entry
	Exit  Exit

	activeMakerOrders *bbgo.ActiveOrderBook
	orderStore        *bbgo.OrderStore
	tradeCollector    *bbgo.TradeCollector

	session *bbgo.ExchangeSession

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
	// session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: types.Interval1d})
}

func (s *Strategy) placeMarketSell(ctx context.Context, orderExecutor bbgo.OrderExecutor) {
	quantity := s.Entry.Quantity
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

	sideEffect := s.Entry.MarginSideEffect
	if len(sideEffect) == 0 {
		sideEffect = types.SideEffectTypeMarginBuy
	}

	submitOrder := types.SubmitOrder{
		Symbol:           s.Symbol,
		Side:             types.SideTypeSell,
		Type:             types.OrderTypeMarket,
		Quantity:         quantity,
		MarginSideEffect: sideEffect,
	}

	s.submitOrders(ctx, orderExecutor, submitOrder)
}

func (s *Strategy) placeOrder(ctx context.Context, lastLow fixedpoint.Value, limitPrice fixedpoint.Value, currentPrice fixedpoint.Value, qty fixedpoint.Value, orderExecutor bbgo.OrderExecutor) {
	submitOrder := types.SubmitOrder{
		Symbol:   s.Symbol,
		Side:     types.SideTypeSell,
		Type:     types.OrderTypeLimit,
		Price:    limitPrice,
		Quantity: qty,
	}
	if !lastLow.IsZero() && s.Entry.Immediate && lastLow.Compare(currentPrice) <= 0 {
		submitOrder.Type = types.OrderTypeMarket
	}
	if s.session.Margin {
		submitOrder.MarginSideEffect = s.Entry.MarginSideEffect
	}

	s.submitOrders(ctx, orderExecutor, submitOrder)
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
	if s.session.Margin {
		submitOrder.MarginSideEffect = s.Exit.MarginSideEffect
	}

	// s.Notify("Submitting %s %s order to close position by %v", s.Symbol, side.String(), percentage, submitOrder)

	createdOrders, err := s.session.Exchange.SubmitOrders(ctx, submitOrder)
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

// check if position can be close or not
func canClosePosition(position *types.Position, signal fixedpoint.Value, price fixedpoint.Value) bool {
	return !signal.IsZero() && position.IsShort() && !position.IsDust(price)
}

// findHigherPivotLow checks the pivot low prices and return the low that is higher than the current price
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
	if !s.TotalQuantity.IsZero() {
		q = s.TotalQuantity.Div(numLayers)
	}

	for i := 0; i < s.Entry.NumLayers; i++ {
		balances := s.session.GetAccount().Balances()
		quoteBalance, _ := balances[s.Market.QuoteCurrency]
		baseBalance, _ := balances[s.Market.BaseCurrency]

		p := limitPrice.Mul(fixedpoint.One.Add(s.Entry.CatBounceRatio.Sub(fixedpoint.NewFromFloat(d.Float64() * float64(i)))))

		if futuresMode {
			// log.Infof("futures mode on")
			if q.Mul(p).Compare(quoteBalance.Available) <= 0 {
				s.placeOrder(ctx, lastLow, p, currentPrice, q, orderExecutor)
			}
		} else if s.Environment.IsBackTesting() {
			// log.Infof("spot backtest mode on")
			if q.Compare(baseBalance.Available) <= 0 {
				s.placeOrder(ctx, lastLow, p, currentPrice, q, orderExecutor)
			}
		} else {
			// log.Infof("spot mode on")
			if q.Compare(baseBalance.Available) <= 0 {
				s.placeOrder(ctx, lastLow, p, currentPrice, q, orderExecutor)
			}
		}
	}
}

func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	// initial required information
	s.session = session

	s.activeMakerOrders = bbgo.NewLocalActiveOrderBook(s.Symbol)
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
	st, _ := session.MarketDataStore(s.Symbol)
	s.pivot = &indicator.Pivot{IntervalWindow: iw}
	s.pivot.Bind(st)

	s.LastLow = fixedpoint.Zero

	session.UserDataStream.OnStart(func() {
		if price, ok := session.LastPrice(s.Symbol); ok {
			if limitPrice, ok := s.findHigherPivotLow(price); ok {
				log.Infof("%s placing limit sell start from %f adds up to %f percent with %d layers of orders", s.Symbol, limitPrice.Float64(), s.Entry.CatBounceRatio.Mul(fixedpoint.NewFromInt(100)).Float64(), s.Entry.NumLayers)
				s.placeBounceSellOrders(ctx, s.LastLow, limitPrice, price, orderExecutor)
			}
		}
	})

	session.MarketDataStream.OnKLine(func(kline types.KLine) {
		// TODO: handle stop loss here, faster than closed kline
		_, found := s.findHigherPivotLow(kline.Close)
		if !found && s.Entry.Immediate && (s.Position.IsClosed() || s.Position.IsDust(kline.Close)) {
			s.Notify("price breaks the previous low, submitting market sell to open a short position")
			s.placeMarketSell(ctx, orderExecutor)
		}
	})

	session.MarketDataStream.OnKLineClosed(func(kline types.KLine) {
		if kline.Symbol != s.Symbol || kline.Interval != s.Interval {
			return
		}

		if s.pivot.LastLow() > 0. {
			log.Infof("pivot low signal detected: %f %s", s.pivot.LastLow(), kline.EndTime.Time())
			s.LastLow = fixedpoint.NewFromFloat(s.pivot.LastLow())
		} else {
			if canClosePosition(s.Position, s.LastLow, kline.Close) {
				R := kline.Close.Div(s.Position.AverageCost)
				if R.Compare(fixedpoint.One.Add(s.Exit.StopLossPercentage)) > 0 {
					// SL
					s.Notify("%s SL triggered", s.Symbol)
					s.ClosePosition(ctx, fixedpoint.One)
					s.tradeCollector.Process()
				} else if R.Compare(fixedpoint.One.Sub(s.Exit.TakeProfitPercentage)) < 0 {
					// TP
					s.Notify("%s TP triggered", s.Symbol)
					s.ClosePosition(ctx, fixedpoint.One)
				} else if kline.GetLowerShadowHeight().Div(kline.Close).Compare(s.Exit.ShadowTPRatio) > 0 {
					// shadow TP
					s.Notify("%s shadow TP triggered", s.Symbol)
					s.ClosePosition(ctx, fixedpoint.One)
				}
			}
			s.LastLow = fixedpoint.Zero
		}

		if !s.LastLow.IsZero() {

			s.pivotLowPrices = append(s.pivotLowPrices, s.LastLow)

			if err := s.activeMakerOrders.GracefulCancel(ctx, s.session.Exchange); err != nil {
				log.WithError(err).Errorf("graceful cancel order error")
			}

			if limitPrice, ok := s.findHigherPivotLow(kline.Close); ok {
				log.Infof("%s place limit sell start from %f adds up to %f percent with %d layers of orders", s.Symbol, limitPrice.Float64(), s.Entry.CatBounceRatio.Mul(fixedpoint.NewFromInt(100)).Float64(), s.Entry.NumLayers)
				s.placeBounceSellOrders(ctx, s.LastLow, limitPrice, kline.Close, orderExecutor)
			}
		}
	})

	return nil
}
