package fib

import (
	"context"
	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/sirupsen/logrus"
	"time"
)

const ID = "fib"

const One = fixedpoint.Value(1e8)
const Two = fixedpoint.Value(2e8)
const Three = fixedpoint.Value(3e8)
const Five = fixedpoint.Value(5e8)
const Six = fixedpoint.Value(6e8)
const Seven = fixedpoint.Value(7e8)

var log = logrus.WithField("strategy", ID)

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

type Strategy struct {
	Symbol   string           `json:"symbol"`
	Interval types.Interval   `json:"interval"`
	Quantity fixedpoint.Value `json:"quantity"`
	hh       types.KLine      //fixedpoint.Value
	ll       types.KLine      //fixedpoint.Value

	activeMakerOrders *bbgo.LocalActiveOrderBook
	orderStore        *bbgo.OrderStore
	session           *bbgo.ExchangeSession
	book              *types.StreamOrderBook
	market            types.Market
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	log.Infof("subscribe %s", s.Symbol)
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.Interval.String()})
}

var Ten = fixedpoint.NewFromInt(10)

func (s *Strategy) placeOrders(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession, midPrice fixedpoint.Value, kline *types.KLine) {

	if kline.High.Compare(s.hh.High) > 0.0 {
		s.hh = *kline
		if err := s.activeMakerOrders.GracefulCancel(ctx, s.session.Exchange); err != nil {
			log.WithError(err).Errorf("graceful cancel order error")
		}
	}
	if kline.Low.Compare(s.ll.Low) < 0.0 {
		s.ll = *kline
		if err := s.activeMakerOrders.GracefulCancel(ctx, s.session.Exchange); err != nil {
			log.WithError(err).Errorf("graceful cancel order error")
		}
	}

	if s.hh.EndTime.Before(kline.EndTime.Time().Add(-250 * time.Minute)) {
		s.hh = *kline
		if err := s.activeMakerOrders.GracefulCancel(ctx, s.session.Exchange); err != nil {
			log.WithError(err).Errorf("graceful cancel order error")
		}
	}
	if s.ll.EndTime.Before(kline.EndTime.Time().Add(-250 * time.Minute)) {
		s.ll = *kline
		if err := s.activeMakerOrders.GracefulCancel(ctx, s.session.Exchange); err != nil {
			log.WithError(err).Errorf("graceful cancel order error")
		}
	}

	t := s.hh.High.Sub(s.ll.Low).Abs()

	log.Infof("fib 0~1 y: %f hh: %f, ll: %f", t.Float64(), s.hh.High.Float64(), s.ll.Low.Float64())

	var buy2, buy3, buy4, buy5, sell2, sell3, sell4, sell5 fixedpoint.Value // buy1, sell1,

	// -0.114
	sell5 = s.hh.High.Sub(t.Mul(fixedpoint.NewFromFloat(-0.114)))

	// 0.0
	sell4 = s.hh.High.Sub(t.Mul(fixedpoint.NewFromFloat(0.0)))

	// 0.1
	sell3 = s.hh.High.Sub(t.Mul(fixedpoint.NewFromFloat(0.114)))

	// 0.236
	sell2 = s.hh.High.Sub(t.Mul(fixedpoint.NewFromFloat(0.236)))

	// 0.382
	//sell1 = s.hh.High.Sub(t.Mul(fixedpoint.NewFromFloat(0.382)))

	// 0.618
	//buy1 = s.hh.High.Sub(t.Mul(fixedpoint.NewFromFloat(0.618)))

	// 0.786
	buy2 = s.hh.High.Sub(t.Mul(fixedpoint.NewFromFloat(0.786)))

	// 0.886
	buy3 = s.hh.High.Sub(t.Mul(fixedpoint.NewFromFloat(0.886)))

	// 1.0
	buy4 = s.hh.High.Sub(t.Mul(fixedpoint.NewFromFloat(1.0)))

	// 1.114
	buy5 = s.hh.High.Sub(t.Mul(fixedpoint.NewFromFloat(1.114)))

	var submitOrders []types.SubmitOrder

	log.Infof("Sell (0.236): %f, Buy (0.786): %f", sell2.Float64(), buy2.Float64())
	//buyOrder1 := types.SubmitOrder{
	//	Symbol:   kline.Symbol,
	//	Side:     types.SideTypeBuy,
	//	Type:     types.OrderTypeLimitMaker,
	//	Price:    buy1,
	//	Quantity: s.Quantity.Mul(One),
	//}
	//submitOrders = append(submitOrders, buyOrder1)
	buyOrder2 := types.SubmitOrder{
		Symbol:   kline.Symbol,
		Side:     types.SideTypeBuy,
		Type:     types.OrderTypeLimit,
		Price:    buy2,
		Quantity: s.Quantity.Mul(One),
	}
	submitOrders = append(submitOrders, buyOrder2)
	buyOrder3 := types.SubmitOrder{
		Symbol:   kline.Symbol,
		Side:     types.SideTypeBuy,
		Type:     types.OrderTypeLimit,
		Price:    buy3,
		Quantity: s.Quantity.Mul(Two),
	}
	submitOrders = append(submitOrders, buyOrder3)
	buyOrder4 := types.SubmitOrder{
		Symbol:   kline.Symbol,
		Side:     types.SideTypeBuy,
		Type:     types.OrderTypeLimit,
		Price:    buy4,
		Quantity: s.Quantity.Mul(Three),
	}
	submitOrders = append(submitOrders, buyOrder4)
	buyOrder5 := types.SubmitOrder{
		Symbol:   kline.Symbol,
		Side:     types.SideTypeBuy,
		Type:     types.OrderTypeLimit,
		Price:    buy5,
		Quantity: s.Quantity.Mul(Six),
	}
	submitOrders = append(submitOrders, buyOrder5)

	//sellOrder1 := types.SubmitOrder{
	//	Symbol:   kline.Symbol,
	//	Side:     types.SideTypeSell,
	//	Type:     types.OrderTypeLimitMaker,
	//	Price:    sell1,
	//	Quantity: s.Quantity.Mul(One),
	//}
	//submitOrders = append(submitOrders, sellOrder1)
	sellOrder2 := types.SubmitOrder{
		Symbol:   kline.Symbol,
		Side:     types.SideTypeSell,
		Type:     types.OrderTypeLimit,
		Price:    sell2,
		Quantity: s.Quantity.Mul(One),
	}
	submitOrders = append(submitOrders, sellOrder2)
	sellOrder3 := types.SubmitOrder{
		Symbol:   kline.Symbol,
		Side:     types.SideTypeSell,
		Type:     types.OrderTypeLimit,
		Price:    sell3,
		Quantity: s.Quantity.Mul(Two),
	}
	submitOrders = append(submitOrders, sellOrder3)
	sellOrder4 := types.SubmitOrder{
		Symbol:   kline.Symbol,
		Side:     types.SideTypeSell,
		Type:     types.OrderTypeLimit,
		Price:    sell4,
		Quantity: s.Quantity.Mul(Three),
	}
	submitOrders = append(submitOrders, sellOrder4)
	sellOrder5 := types.SubmitOrder{
		Symbol:   kline.Symbol,
		Side:     types.SideTypeSell,
		Type:     types.OrderTypeLimit,
		Price:    sell5,
		Quantity: s.Quantity.Mul(Six),
	}
	submitOrders = append(submitOrders, sellOrder5)

	createdOrders, err := orderExecutor.SubmitOrders(ctx, submitOrders...)
	if err != nil {
		log.WithError(err).Errorf("can not place orders")
	}

	s.orderStore.Add(createdOrders...)
	s.activeMakerOrders.Add(createdOrders...)
}

// This strategy simply spent all available quote currency to buy the symbol whenever kline gets closed
func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	// initial required information
	s.session = session

	s.activeMakerOrders = bbgo.NewLocalActiveOrderBook(s.Symbol)
	s.activeMakerOrders.BindStream(session.UserDataStream)

	s.orderStore = bbgo.NewOrderStore(s.Symbol)
	s.orderStore.BindStream(session.UserDataStream)

	session.UserDataStream.OnStart(func() {
		log.Infof("connected")
		//if price, ok := session.LastPrice(s.Symbol); ok {
		//s.placeOrders(ctx, orderExecutor, price, nil)
		//}
	})

	session.MarketDataStream.OnKLineClosed(func(kline types.KLine) {
		if kline.Symbol != s.Symbol || kline.Interval != s.Interval {
			return
		}

		//if err := s.activeMakerOrders.GracefulCancel(ctx, s.session.Exchange); err != nil {
		//	log.WithError(err).Errorf("graceful cancel order error")
		//}

		// check if there is a canceled order had partially filled.
		//s.tradeCollector.Process()

		s.placeOrders(ctx, orderExecutor, session, kline.Close, &kline)
	})

	return nil
}
