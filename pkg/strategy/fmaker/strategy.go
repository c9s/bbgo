package fmaker

import (
	"context"
	"fmt"
	"math"

	"github.com/sajari/regression"
	"github.com/sirupsen/logrus"
	"gonum.org/v1/gonum/floats"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/core"
	floats2 "github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

const ID = "fmaker"

var fifteen = fixedpoint.NewFromInt(15)
var three = fixedpoint.NewFromInt(3)
var two = fixedpoint.NewFromInt(2)

var log = logrus.WithField("strategy", ID)

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

type IntervalWindowSetting struct {
	types.IntervalWindow
}

type Strategy struct {
	Environment *bbgo.Environment
	Symbol      string `json:"symbol"`
	Market      types.Market
	Interval    types.Interval   `json:"interval"`
	Quantity    fixedpoint.Value `json:"quantity"`

	// persistence fields
	Position    *types.Position    `json:"position,omitempty" persistence:"position"`
	ProfitStats *types.ProfitStats `json:"profitStats,omitempty" persistence:"profit_stats"`

	Spread fixedpoint.Value `json:"spread" persistence:"spread"`

	activeMakerOrders *bbgo.ActiveOrderBook
	// closePositionOrders *bbgo.LocalActiveOrderBook

	orderStore     *core.OrderStore
	tradeCollector *core.TradeCollector

	session *bbgo.ExchangeSession

	bbgo.QuantityOrAmount

	S0 *S0
	S1 *S1
	S2 *S2
	S3 *S3
	S4 *S4
	S5 *S5
	S6 *S6
	S7 *S7

	A2  *A2
	A3  *A3
	A18 *A18
	A34 *A34

	R *R

	// StrategyController
	bbgo.StrategyController
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	log.Infof("subscribe %s", s.Symbol)
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.Interval})
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: types.Interval15m})

}

func (s *Strategy) placeOrder(ctx context.Context, price fixedpoint.Value, qty fixedpoint.Value, orderExecutor bbgo.OrderExecutor) {
	submitOrder := types.SubmitOrder{
		Symbol:   s.Symbol,
		Side:     types.SideTypeSell,
		Type:     types.OrderTypeLimit,
		Price:    price,
		Quantity: qty,
	}
	createdOrders, err := orderExecutor.SubmitOrders(ctx, submitOrder)
	if err != nil {
		log.WithError(err).Errorf("can not place orders")
	}
	s.orderStore.Add(createdOrders...)
	s.activeMakerOrders.Add(createdOrders...)
	// s.tradeCollector.Process()
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
		// Price:    closePrice,
		Market: s.Market,
	}

	// s.Notify("Submitting %s %s order to close position by %v", s.Symbol, side.String(), percentage, submitOrder)

	createdOrder, err := s.session.Exchange.SubmitOrder(ctx, submitOrder)
	if err != nil {
		log.WithError(err).Errorf("can not place position close order")
	} else if createdOrder != nil {
		s.orderStore.Add(*createdOrder)
		s.activeMakerOrders.Add(*createdOrder)
	}

	return err
}
func (s *Strategy) InstanceID() string {
	return fmt.Sprintf("%s:%s", ID, s.Symbol)
}

func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	// initial required information
	s.session = session
	// s.prevClose = fixedpoint.Zero

	// first we need to get market data store(cached market data) from the exchange session
	// st, _ := session.MarketDataStore(s.Symbol)

	s.activeMakerOrders = bbgo.NewActiveOrderBook(s.Symbol)
	s.activeMakerOrders.BindStream(session.UserDataStream)

	// s.closePositionOrders = bbgo.NewLocalActiveOrderBook(s.Symbol)
	// s.closePositionOrders.BindStream(session.UserDataStream)

	s.orderStore = core.NewOrderStore(s.Symbol)
	s.orderStore.BindStream(session.UserDataStream)

	if s.Position == nil {
		s.Position = types.NewPositionFromMarket(s.Market)
	}

	// calculate group id for orders
	instanceID := s.InstanceID()
	// s.groupID = util.FNV32(instanceID)

	// Always update the position fields
	s.Position.Strategy = ID
	s.Position.StrategyInstanceID = instanceID

	s.tradeCollector = core.NewTradeCollector(s.Symbol, s.Position, s.orderStore)
	s.tradeCollector.OnTrade(func(trade types.Trade, profit, netProfit fixedpoint.Value) {
		// StrategyController
		if s.Status != types.StrategyStatusRunning {
			return
		}

		bbgo.Notify(trade)
		s.ProfitStats.AddTrade(trade)

		if profit.Compare(fixedpoint.Zero) == 0 {
			s.Environment.RecordPosition(s.Position, trade, nil)
		} else {
			log.Infof("%s generated profit: %v", s.Symbol, profit)
			p := s.Position.NewProfit(trade, profit, netProfit)
			p.Strategy = ID
			p.StrategyInstanceID = instanceID
			bbgo.Notify(&p)

			s.ProfitStats.AddProfit(p)
			bbgo.Notify(&s.ProfitStats)

			s.Environment.RecordPosition(s.Position, trade, &p)
		}
	})

	s.tradeCollector.OnPositionUpdate(func(position *types.Position) {
		log.Infof("position changed: %s", s.Position)
		bbgo.Notify(s.Position)
	})
	s.tradeCollector.BindStream(session.UserDataStream)
	st, _ := session.MarketDataStore(s.Symbol)

	riw := types.IntervalWindow{Window: 1, Interval: s.Interval}
	s.R = &R{IntervalWindow: riw}
	s.R.Bind(st)

	s0iw := types.IntervalWindow{Window: 20, Interval: s.Interval}
	s.S0 = &S0{IntervalWindow: s0iw}
	s.S0.Bind(st)

	s1iw := types.IntervalWindow{Window: 20, Interval: s.Interval}
	s.S1 = &S1{IntervalWindow: s1iw}
	s.S1.Bind(st)

	s2iw := types.IntervalWindow{Window: 20, Interval: s.Interval}
	s.S2 = &S2{IntervalWindow: s2iw}
	s.S2.Bind(st)

	s3iw := types.IntervalWindow{Window: 2, Interval: s.Interval}
	s.S3 = &S3{IntervalWindow: s3iw}
	s.S3.Bind(st)

	s4iw := types.IntervalWindow{Window: 2, Interval: s.Interval}
	s.S4 = &S4{IntervalWindow: s4iw}
	s.S4.Bind(st)

	s5iw := types.IntervalWindow{Window: 10, Interval: s.Interval}
	s.S5 = &S5{IntervalWindow: s5iw}
	s.S5.Bind(st)

	s6iw := types.IntervalWindow{Window: 2, Interval: s.Interval}
	s.S6 = &S6{IntervalWindow: s6iw}
	s.S6.Bind(st)

	s7iw := types.IntervalWindow{Window: 2, Interval: s.Interval}
	s.S7 = &S7{IntervalWindow: s7iw}
	s.S7.Bind(st)

	a2iw := types.IntervalWindow{Window: 2, Interval: s.Interval}
	s.A2 = &A2{IntervalWindow: a2iw}
	s.A2.Bind(st)

	a3iw := types.IntervalWindow{Window: 8, Interval: s.Interval}
	s.A3 = &A3{IntervalWindow: a3iw}
	s.A3.Bind(st)

	a18iw := types.IntervalWindow{Window: 5, Interval: s.Interval}
	s.A18 = &A18{IntervalWindow: a18iw}
	s.A18.Bind(st)

	a34iw := types.IntervalWindow{Window: 12, Interval: s.Interval}
	s.A34 = &A34{IntervalWindow: a34iw}
	s.A34.Bind(st)

	session.UserDataStream.OnStart(func() {
		log.Infof("connected")
	})

	outlook := 1

	// futuresMode := s.session.Futures || s.session.IsolatedFutures
	cnt := 0

	// var prevEr float64
	session.MarketDataStream.OnKLineClosed(func(kline types.KLine) {

		// if kline.Interval == types.Interval15m && kline.Symbol == s.Symbol && !s.Market.IsDustQuantity(s.Position.GetBase(), kline.Close) {
		//	if err := s.activeMakerOrders.GracefulCancel(ctx, s.session.Exchange); err != nil {
		//		log.WithError(err).Errorf("graceful cancel order error")
		//	}
		//	s.ClosePosition(ctx, fixedpoint.One)
		//	s.tradeCollector.Process()
		// }
		if kline.Symbol != s.Symbol || kline.Interval != s.Interval {
			return
		}

		if err := s.activeMakerOrders.GracefulCancel(ctx, s.session.Exchange); err != nil {
			log.WithError(err).Errorf("graceful cancel order error")
		}

		cnt += 1
		if cnt < 15+1+outlook {
			return
		}

		r := new(regression.Regression)
		r.SetObserved("Return Rate Per Interval")
		r.SetVar(0, "S0")
		r.SetVar(1, "S1")
		r.SetVar(2, "S2")
		// r.SetVar(2, "S3")
		r.SetVar(3, "S4")
		r.SetVar(4, "S5")
		r.SetVar(5, "S6")
		r.SetVar(6, "S7")
		r.SetVar(7, "A2")
		r.SetVar(8, "A3")
		r.SetVar(9, "A18")
		r.SetVar(10, "A34")

		var rdps regression.DataPoints

		for i := 1; i <= 15; i++ {
			s0 := s.S0.Values[len(s.S0.Values)-i-outlook]
			s1 := s.S1.Values[len(s.S1.Values)-i-outlook]
			s2 := s.S2.Values[len(s.S2.Values)-i-outlook]
			// s3 := s.S3.Values[len(s.S3.Values)-i-1]
			s4 := s.S4.Values[len(s.S4.Values)-i-outlook]
			s5 := s.S5.Values[len(s.S5.Values)-i-outlook]
			s6 := s.S6.Values[len(s.S6.Values)-i-outlook]
			s7 := s.S7.Values[len(s.S7.Values)-i-outlook]
			a2 := s.A2.Values[len(s.A2.Values)-i-outlook]
			a3 := s.A3.Values[len(s.A3.Values)-i-outlook]
			a18 := s.A18.Values[len(s.A18.Values)-i-outlook]
			a34 := s.A34.Values[len(s.A34.Values)-i-outlook]

			ret := s.R.Values[len(s.R.Values)-i]
			rdps = append(rdps, regression.DataPoint(ret, floats2.Slice{s0, s1, s2, s4, s5, s6, s7, a2, a3, a18, a34}))
		}
		// for i := 40; i > 20; i-- {
		//	s0 := preprocessing(s.S0.Values[len(s.S0.Values)-i : len(s.S0.Values)-i+20-outlook])
		//	s1 := preprocessing(s.S1.Values[len(s.S1.Values)-i : len(s.S1.Values)-i+20-outlook])
		//	s2 := preprocessing(s.S2.Values[len(s.S2.Values)-i : len(s.S2.Values)-i+20-outlook])
		//	//s3 := s.S3.Values[len(s.S3.Values)-i-1]
		//	s4 := preprocessing(s.S4.Values[len(s.S4.Values)-i : len(s.S4.Values)-i+20-outlook])
		//	s5 := preprocessing(s.S5.Values[len(s.S5.Values)-i : len(s.S5.Values)-i+20-outlook])
		//	a2 := preprocessing(s.A2.Values[len(s.A2.Values)-i : len(s.A2.Values)-i+20-outlook])
		//	a3 := preprocessing(s.A3.Values[len(s.A3.Values)-i : len(s.A3.Values)-i+20-outlook])
		//	a18 := preprocessing(s.A18.Values[len(s.A18.Values)-i : len(s.A18.Values)-i+20-outlook])
		//	a34 := preprocessing(s.A18.Values[len(s.A18.Values)-i : len(s.A18.Values)-i+20-outlook])
		//
		//	ret := s.R.Values[len(s.R.Values)-i]
		//	rdps = append(rdps, regression.DataPoint(ret, types.Float64Slice{s0, s1, s2, s4, s5, a2, a3, a18, a34}))
		// }
		r.Train(rdps...)
		r.Run()
		er, _ := r.Predict(floats2.Slice{s.S0.Last(0), s.S1.Last(0), s.S2.Last(0), s.S4.Last(0), s.S5.Last(0), s.S6.Last(0), s.S7.Last(0), s.A2.Last(0), s.A3.Last(0), s.A18.Last(0), s.A34.Last(0)})
		log.Infof("Expected Return Rate: %f", er)

		q := new(regression.Regression)
		q.SetObserved("Order Quantity Per Interval")
		q.SetVar(0, "S0")
		q.SetVar(1, "S1")
		q.SetVar(2, "S2")
		// q.SetVar(2, "S3")
		q.SetVar(3, "S4")
		q.SetVar(4, "S5")
		q.SetVar(5, "S6")
		q.SetVar(6, "S7")
		q.SetVar(7, "A2")
		q.SetVar(8, "A3")
		q.SetVar(9, "A18")
		q.SetVar(10, "A34")

		var qdps regression.DataPoints

		for i := 1; i <= 15; i++ {
			s0 := math.Pow(s.S0.Values[len(s.S0.Values)-i-outlook], 1)
			s1 := math.Pow(s.S1.Values[len(s.S1.Values)-i-outlook], 1)
			s2 := math.Pow(s.S2.Values[len(s.S2.Values)-i-outlook], 1)
			// s3 := s.S3.Values[len(s.S3.Values)-i-1]
			s4 := math.Pow(s.S4.Values[len(s.S4.Values)-i-outlook], 1)
			s5 := math.Pow(s.S5.Values[len(s.S5.Values)-i-outlook], 1)
			s6 := s.S6.Values[len(s.S6.Values)-i-outlook]
			s7 := s.S7.Values[len(s.S7.Values)-i-outlook]
			a2 := math.Pow(s.A2.Values[len(s.A2.Values)-i-outlook], 1)
			a3 := math.Pow(s.A3.Values[len(s.A3.Values)-i-outlook], 1)
			a18 := math.Pow(s.A18.Values[len(s.A18.Values)-i-outlook], 1)
			a34 := math.Pow(s.A34.Values[len(s.A34.Values)-i-outlook], 1)

			ret := s.R.Values[len(s.R.Values)-i]
			qty := math.Abs(ret)
			qdps = append(qdps, regression.DataPoint(qty, floats2.Slice{s0, s1, s2, s4, s5, s6, s7, a2, a3, a18, a34}))
		}
		// for i := 40; i > 20; i-- {
		//	s0 := preprocessing(s.S0.Values[len(s.S0.Values)-i : len(s.S0.Values)-i+20-outlook])
		//	s1 := preprocessing(s.S1.Values[len(s.S1.Values)-i : len(s.S1.Values)-i+20-outlook])
		//	s2 := preprocessing(s.S2.Values[len(s.S2.Values)-i : len(s.S2.Values)-i+20-outlook])
		//	//s3 := s.S3.Values[len(s.S3.Values)-i-1]
		//	s4 := preprocessing(s.S4.Values[len(s.S4.Values)-i : len(s.S4.Values)-i+20-outlook])
		//	s5 := preprocessing(s.S5.Values[len(s.S5.Values)-i : len(s.S5.Values)-i+20-outlook])
		//	a2 := preprocessing(s.A2.Values[len(s.A2.Values)-i : len(s.A2.Values)-i+20-outlook])
		//	a3 := preprocessing(s.A3.Values[len(s.A3.Values)-i : len(s.A3.Values)-i+20-outlook])
		//	a18 := preprocessing(s.A18.Values[len(s.A18.Values)-i : len(s.A18.Values)-i+20-outlook])
		//	a34 := preprocessing(s.A18.Values[len(s.A18.Values)-i : len(s.A18.Values)-i+20-outlook])
		//
		//	ret := s.R.Values[len(s.R.Values)-i]
		//	qty := math.Abs(ret)
		//	qdps = append(qdps, regression.DataPoint(qty, types.Float64Slice{s0, s1, s2, s4, s5, a2, a3, a18, a34}))
		// }
		q.Train(qdps...)

		q.Run()

		log.Info(s.S0.Last(0), s.S1.Last(0), s.S2.Last(0), s.S3.Last(0), s.S4.Last(0), s.S5.Last(0), s.S6.Last(0), s.S7.Last(0), s.A2.Last(0), s.A3.Last(0), s.A18.Last(0), s.A34.Last(0))

		log.Infof("Return Rate Regression formula:\n%v", r.Formula)
		log.Infof("Order Quantity Regression formula:\n%v", q.Formula)

		// s0 := preprocessing(s.S0.Values[len(s.S0.Values)-20 : len(s.S0.Values)-1])
		// s1 := preprocessing(s.S1.Values[len(s.S1.Values)-20 : len(s.S1.Values)-1-outlook])
		// s2 := preprocessing(s.S2.Values[len(s.S2.Values)-20 : len(s.S2.Values)-1-outlook])
		// //s3 := s.S3.Values[len(s.S3.Values)-i-1]
		// s4 := preprocessing(s.S4.Values[len(s.S4.Values)-20 : len(s.S4.Values)-1-outlook])
		// s5 := preprocessing(s.S5.Values[len(s.S5.Values)-20 : len(s.S5.Values)-1-outlook])
		// a2 := preprocessing(s.A2.Values[len(s.A2.Values)-20 : len(s.A2.Values)-1-outlook])
		// a3 := preprocessing(s.A3.Values[len(s.A3.Values)-20 : len(s.A3.Values)-1-outlook])
		// a18 := preprocessing(s.A18.Values[len(s.A18.Values)-20 : len(s.A18.Values)-1-outlook])
		// a34 := preprocessing(s.A18.Values[len(s.A18.Values)-20 : len(s.A18.Values)-1-outlook])
		// er, _ := r.Predict(types.Float64Slice{s0, s1, s2, s4, s5, a2, a3, a18, a34})
		// eq, _ := q.Predict(types.Float64Slice{s0, s1, s2, s4, s5, a2, a3, a18, a34})
		eq, _ := q.Predict(floats2.Slice{s.S0.Last(0), s.S1.Last(0), s.S2.Last(0), s.S4.Last(0), s.S5.Last(0), s.S6.Last(0), s.S7.Last(0), s.A2.Last(0), s.A3.Last(0), s.A18.Last(0), s.A34.Last(0), er})
		log.Infof("Expected Order Quantity: %f", eq)
		// if float64(s.Position.GetBase().Sign())*er < 0 {
		//	s.ClosePosition(ctx, fixedpoint.One, kline.Close)
		//	s.tradeCollector.Process()
		// }
		// prevEr = er

		// spd := s.Spread.Float64()

		// inventory = m * alpha + spread
		AskAlphaBoundary := (s.Position.GetBase().Mul(kline.Close).Float64() - 100) / 10000
		BidAlphaBoundary := (s.Position.GetBase().Mul(kline.Close).Float64() + 100) / 10000

		log.Info(s.Position.GetBase().Mul(kline.Close).Float64(), AskAlphaBoundary, er, BidAlphaBoundary)

		BidPrice := kline.Close.Mul(fixedpoint.One.Sub(s.Spread))
		BidQty := s.QuantityOrAmount.CalculateQuantity(BidPrice)
		BidQty = BidQty // .Mul(fixedpoint.One.Add(fixedpoint.NewFromFloat(eq)))

		AskPrice := kline.Close.Mul(fixedpoint.One.Add(s.Spread))
		AskQty := s.QuantityOrAmount.CalculateQuantity(AskPrice)
		AskQty = AskQty // .Mul(fixedpoint.One.Add(fixedpoint.NewFromFloat(eq)))

		if er > 0 || (er < 0 && er > AskAlphaBoundary/kline.Close.Float64()) {
			submitOrder := types.SubmitOrder{
				Symbol:   s.Symbol,
				Side:     types.SideTypeBuy,
				Type:     types.OrderTypeLimitMaker,
				Price:    BidPrice,
				Quantity: BidQty, // 0.0005
			}
			createdOrders, err := orderExecutor.SubmitOrders(ctx, submitOrder)
			if err != nil {
				log.WithError(err).Errorf("can not place orders")
			}
			s.orderStore.Add(createdOrders...)
			s.activeMakerOrders.Add(createdOrders...)
			s.tradeCollector.Process()

			// submitOrder = types.SubmitOrder{
			//	Symbol:   s.Symbol,
			//	Side:     types.SideTypeSell,
			//	Type:     types.OrderTypeLimitMaker,
			//	Price:    kline.Close.Mul(fixedpoint.One.Add(s.Spread)),
			//	Quantity: fixedpoint.NewFromFloat(math.Max(math.Min(eq, 0.003), 0.0005)), //0.0005
			// }
			// createdOrders, err = orderExecutor.SubmitOrder(ctx, submitOrder)
			// if err != nil {
			//	log.WithError(err).Errorf("can not place orders")
			// }
			// s.orderStore.Add(createdOrders...)
			// s.activeMakerOrders.Add(createdOrders...)
			// s.tradeCollector.Process()
		}
		if er < 0 || (er > 0 && er < BidAlphaBoundary/kline.Close.Float64()) {
			submitOrder := types.SubmitOrder{
				Symbol:   s.Symbol,
				Side:     types.SideTypeSell,
				Type:     types.OrderTypeLimitMaker,
				Price:    AskPrice,
				Quantity: AskQty, // 0.0005
			}
			createdOrders, err := orderExecutor.SubmitOrders(ctx, submitOrder)
			if err != nil {
				log.WithError(err).Errorf("can not place orders")
			}
			s.orderStore.Add(createdOrders...)
			s.activeMakerOrders.Add(createdOrders...)
			s.tradeCollector.Process()

			// submitOrder = types.SubmitOrder{
			//	Symbol:   s.Symbol,
			//	Side:     types.SideTypeBuy,
			//	Type:     types.OrderTypeLimitMaker,
			//	Price:    kline.Close.Mul(fixedpoint.One.Sub(s.Spread)),
			//	Quantity: fixedpoint.NewFromFloat(math.Max(math.Min(eq, 0.003), 0.0005)), //0.0005
			// }
			// createdOrders, err = orderExecutor.SubmitOrder(ctx, submitOrder)
			// if err != nil {
			//	log.WithError(err).Errorf("can not place orders")
			// }
			// s.orderStore.Add(createdOrders...)
			// s.activeMakerOrders.Add(createdOrders...)
			// s.tradeCollector.Process()
		}

	})

	return nil
}

func tanh(x float64) float64 {
	y := (math.Exp(x) - math.Exp(-x)) / (math.Exp(x) + math.Exp(-x))
	return y
}

func mean(xs []float64) float64 {
	return floats.Sum(xs) / float64(len(xs))
}

func stddev(xs []float64) float64 {
	mu := mean(xs)
	squaresum := 0.
	for _, x := range xs {
		squaresum += (x - mu) * (x - mu)
	}
	return math.Sqrt(squaresum / float64(len(xs)-1))
}

func preprocessing(xs []float64) float64 {
	// return 0.5 * tanh(0.01*((xs[len(xs)-1]-mean(xs))/stddev(xs))) // tanh estimator
	return tanh((xs[len(xs)-1] - mean(xs)) / stddev(xs)) // tanh z-score
	return (xs[len(xs)-1] - mean(xs)) / stddev(xs)       // z-score
}
