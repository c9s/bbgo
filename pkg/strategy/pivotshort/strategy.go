package pivotshort

import (
	"context"
	"fmt"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/sirupsen/logrus"
)

const ID = "pivotshort"

var log = logrus.WithField("strategy", ID)

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

type IntervalWindowSetting struct {
	types.IntervalWindow
}

type Strategy struct {
	*bbgo.Graceful
	*bbgo.Notifiability
	*bbgo.Persistence

	Environment   *bbgo.Environment
	Symbol        string `json:"symbol"`
	Market        types.Market
	Interval      types.Interval   `json:"interval"`
	Quantity      fixedpoint.Value `json:"quantity"`
	TotalQuantity fixedpoint.Value `json:"totalQuantity"`

	// persistence fields
	Position    *types.Position    `json:"position,omitempty" persistence:"position"`
	ProfitStats *types.ProfitStats `json:"profitStats,omitempty" persistence:"profit_stats"`

	PivotLength           int                             `json:"pivotLength"`
	StopLossRatio         fixedpoint.Value                `json:"stopLossRatio"`
	TakeProfitRatio       fixedpoint.Value                `json:"takeProfitRatio"`
	CatBounceRatio        fixedpoint.Value                `json:"catBounceRatio"`
	NumLayers             fixedpoint.Value                `json:"numLayers"`
	ShadowTPRatio         fixedpoint.Value                `json:"shadowTPRatio"`
	MarginOrderSideEffect types.MarginOrderSideEffectType `json:"marginOrderSideEffect"`

	activeMakerOrders *bbgo.LocalActiveOrderBook
	orderStore        *bbgo.OrderStore
	tradeCollector    *bbgo.TradeCollector

	session *bbgo.ExchangeSession

	pivot *indicator.Pivot

	// StrategyController
	bbgo.StrategyController
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	log.Infof("subscribe %s", s.Symbol)
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.Interval})
	//session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: types.Interval1d})
}

func (s *Strategy) placeOrder(ctx context.Context, price fixedpoint.Value, qty fixedpoint.Value, orderExecutor bbgo.OrderExecutor) {
	submitOrder := types.SubmitOrder{
		Symbol:   s.Symbol,
		Side:     types.SideTypeSell,
		Type:     types.OrderTypeLimit,
		Price:    price,
		Quantity: qty,
	}
	if s.session.Margin {
		submitOrder.MarginSideEffect = s.MarginOrderSideEffect
	}

	createdOrders, err := orderExecutor.SubmitOrders(ctx, submitOrder)
	if err != nil {
		log.WithError(err).Errorf("can not place orders")
	}
	s.orderStore.Add(createdOrders...)
	s.activeMakerOrders.Add(createdOrders...)
	//s.tradeCollector.Process()
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
		submitOrder.MarginSideEffect = s.MarginOrderSideEffect
	}

	//s.Notify("Submitting %s %s order to close position by %v", s.Symbol, side.String(), percentage, submitOrder)

	createdOrders, err := s.session.Exchange.SubmitOrders(ctx, submitOrder)
	if err != nil {
		log.WithError(err).Errorf("can not place position close order")
	}

	s.orderStore.Add(createdOrders...)
	s.activeMakerOrders.Add(createdOrders...)
	return err
}
func (s *Strategy) InstanceID() string {
	return fmt.Sprintf("%s:%s", ID, s.Symbol)
}

func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	// initial required information
	s.session = session
	//s.prevClose = fixedpoint.Zero

	// first we need to get market data store(cached market data) from the exchange session
	//st, _ := session.MarketDataStore(s.Symbol)

	s.activeMakerOrders = bbgo.NewLocalActiveOrderBook(s.Symbol)
	s.activeMakerOrders.BindStream(session.UserDataStream)

	s.orderStore = bbgo.NewOrderStore(s.Symbol)
	s.orderStore.BindStream(session.UserDataStream)

	if s.Position == nil {
		s.Position = types.NewPositionFromMarket(s.Market)
	}

	// calculate group id for orders
	instanceID := s.InstanceID()
	//s.groupID = util.FNV32(instanceID)

	// Always update the position fields
	s.Position.Strategy = ID
	s.Position.StrategyInstanceID = instanceID

	s.tradeCollector = bbgo.NewTradeCollector(s.Symbol, s.Position, s.orderStore)
	s.tradeCollector.OnTrade(func(trade types.Trade, profit, netProfit fixedpoint.Value) {
		// StrategyController
		if s.Status != types.StrategyStatusRunning {
			return
		}

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

	session.UserDataStream.OnStart(func() {
		log.Infof("connected")
	})

	var lastLow fixedpoint.Value
	futuresMode := s.session.Futures || s.session.IsolatedFutures
	d := s.CatBounceRatio.Div(s.NumLayers)
	q := s.Quantity
	if !s.TotalQuantity.IsZero() {
		q = s.TotalQuantity.Div(s.NumLayers)
	}

	var pivotBuffer []fixedpoint.Value

	session.MarketDataStream.OnKLineClosed(func(kline types.KLine) {
		if kline.Symbol != s.Symbol || kline.Interval != s.Interval {
			return
		}

		if s.pivot.LastLow() > 0. {
			log.Info(s.pivot.LastLow(), kline.EndTime)
			lastLow = fixedpoint.NewFromFloat(s.pivot.LastLow())
		} else {
			if !lastLow.IsZero() && s.Position.IsShort() && !s.Position.IsDust(kline.Close) {
				R := kline.Close.Div(s.Position.AverageCost)
				if R.Compare(fixedpoint.One.Add(s.StopLossRatio)) > 0 {
					// SL
					log.Infof("SL triggered")
					s.ClosePosition(ctx, fixedpoint.One)
					s.tradeCollector.Process()
				} else if R.Compare(fixedpoint.One.Sub(s.TakeProfitRatio)) < 0 {
					// TP
					log.Infof("TP triggered")
					s.ClosePosition(ctx, fixedpoint.One)
				} else if kline.GetLowerShadowHeight().Div(kline.Close).Compare(s.ShadowTPRatio) > 0 {
					// shadow TP
					log.Infof("shadow TP triggered")
					s.ClosePosition(ctx, fixedpoint.One)
					s.tradeCollector.Process()
				}
			}
			lastLow = fixedpoint.Zero
		}

		if !lastLow.IsZero() {

			pivotBuffer = append(pivotBuffer, lastLow)

			if err := s.activeMakerOrders.GracefulCancel(ctx, s.session.Exchange); err != nil {
				log.WithError(err).Errorf("graceful cancel order error")
			}

			postPrice := kline.Close
			for l := len(pivotBuffer) - 1; l > 0; l-- {
				if pivotBuffer[l].Compare(kline.Close) > 0 {
					postPrice = pivotBuffer[l]
					break
				}
			}

			for i := 0; i < int(s.NumLayers.Float64()); i++ {
				balances := s.session.GetAccount().Balances()
				quoteBalance, _ := balances[s.Market.QuoteCurrency]
				baseBalance, _ := balances[s.Market.BaseCurrency]

				p := postPrice.Mul(fixedpoint.One.Add(s.CatBounceRatio.Sub(fixedpoint.NewFromFloat(d.Float64() * float64(i)))))
				//
				if futuresMode {
					//log.Infof("futures mode on ")
					if q.Mul(p).Compare(quoteBalance.Available) < 0 {
						s.placeOrder(ctx, p, q, orderExecutor)
						s.tradeCollector.Process()
					}
				} else if s.Environment.IsBackTesting() {
					//log.Infof("spot backtest mode on ")
					if q.Compare(baseBalance.Available) < 0 {
						s.placeOrder(ctx, p, q, orderExecutor)
						s.tradeCollector.Process()
					}
				} else {
					//log.Infof("spot mode on ")
					if q.Compare(baseBalance.Available) < 0 {
						s.placeOrder(ctx, p, q, orderExecutor)
						s.tradeCollector.Process()
					}
				}

			}
			//s.placeOrder(ctx, lastLow.Mul(fixedpoint.One.Add(s.CatBounceRatio)), s.Quantity, orderExecutor)
		}
	})

	return nil
}
