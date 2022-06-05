package factorzoo

import (
	"context"
	"fmt"

	"github.com/sajari/regression"
	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

const ID = "factorzoo"

var three = fixedpoint.NewFromInt(3)

var log = logrus.WithField("strategy", ID)

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

type IntervalWindowSetting struct {
	types.IntervalWindow
}

type Strategy struct {
	Symbol   string `json:"symbol"`
	Market   types.Market
	Interval types.Interval   `json:"interval"`
	Quantity fixedpoint.Value `json:"quantity"`

	Position *types.Position `json:"position,omitempty"`

	activeMakerOrders *bbgo.ActiveOrderBook
	orderStore        *bbgo.OrderStore
	tradeCollector    *bbgo.TradeCollector

	session *bbgo.ExchangeSession
	book    *types.StreamOrderBook

	prevClose fixedpoint.Value

	pvDivergenceSetting *IntervalWindowSetting `json:"pvDivergence"`
	pvDivergence        *Correlation

	Ret   []float64
	Alpha [][]float64

	T      int64
	prevER fixedpoint.Value
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	log.Infof("subscribe %s", s.Symbol)
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.Interval})
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

	// s.Notify("Submitting %s %s order to close position by %v", s.Symbol, side.String(), percentage, submitOrder)

	createdOrders, err := s.session.Exchange.SubmitOrders(ctx, submitOrder)
	if err != nil {
		log.WithError(err).Errorf("can not place position close order")
	}

	s.orderStore.Add(createdOrders...)
	s.activeMakerOrders.Add(createdOrders...)
	return err
}

func (s *Strategy) placeOrders(ctx context.Context, orderExecutor bbgo.OrderExecutor, er fixedpoint.Value) {

	// if s.prevER.Sign() < 0 && er.Sign() > 0 {
	if er.Sign() >= 0 {
		submitOrder := types.SubmitOrder{
			Symbol:   s.Symbol,
			Side:     types.SideTypeBuy,
			Type:     types.OrderTypeMarket,
			Quantity: s.Quantity, // er.Abs().Mul(fixedpoint.NewFromInt(20)),
		}
		createdOrders, err := orderExecutor.SubmitOrders(ctx, submitOrder)
		if err != nil {
			log.WithError(err).Errorf("can not place orders")
		}
		s.orderStore.Add(createdOrders...)
		s.activeMakerOrders.Add(createdOrders...)
		// } else if s.prevER.Sign() > 0 && er.Sign() < 0 {
	} else {
		submitOrder := types.SubmitOrder{
			Symbol:   s.Symbol,
			Side:     types.SideTypeSell,
			Type:     types.OrderTypeMarket,
			Quantity: s.Quantity, // er.Abs().Mul(fixedpoint.NewFromInt(20)),
		}
		createdOrders, err := orderExecutor.SubmitOrders(ctx, submitOrder)
		if err != nil {
			log.WithError(err).Errorf("can not place orders")
		}
		s.orderStore.Add(createdOrders...)
		s.activeMakerOrders.Add(createdOrders...)
	}
	s.prevER = er
}

func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	// initial required information
	s.session = session
	s.prevClose = fixedpoint.Zero

	// first we need to get market data store(cached market data) from the exchange session
	st, _ := session.MarketDataStore(s.Symbol)
	// setup the time frame size
	iw := types.IntervalWindow{Window: 50, Interval: s.Interval}
	// construct CORR indicator
	s.pvDivergence = &Correlation{IntervalWindow: iw}
	// bind indicator to the data store, so that our callback could be triggered
	s.pvDivergence.Bind(st)
	// s.pvDivergence.OnUpdate(func(corr float64) {
	//	//fmt.Printf("now we've got corr: %f\n", corr)
	// })

	s.Alpha = [][]float64{{}, {}, {}, {}, {}}
	s.Ret = []float64{}
	// thetas := []float64{0, 0, 0, 0}
	preCompute := 0

	s.activeMakerOrders = bbgo.NewLocalActiveOrderBook(s.Symbol)
	s.activeMakerOrders.BindStream(session.UserDataStream)

	s.orderStore = bbgo.NewOrderStore(s.Symbol)
	s.orderStore.BindStream(session.UserDataStream)

	if s.Position == nil {
		s.Position = types.NewPositionFromMarket(s.Market)
	}

	s.tradeCollector = bbgo.NewTradeCollector(s.Symbol, s.Position, s.orderStore)
	s.tradeCollector.BindStream(session.UserDataStream)

	session.UserDataStream.OnStart(func() {
		log.Infof("connected")
	})

	session.MarketDataStream.OnKLineClosed(func(kline types.KLine) {

		if kline.Symbol != s.Symbol || kline.Interval != s.Interval {
			return
		}

		if err := s.activeMakerOrders.GracefulCancel(ctx, s.session.Exchange); err != nil {
			log.WithError(err).Errorf("graceful cancel order error")
		}

		// amplitude volume divergence
		corr := fixedpoint.NewFromFloat(s.pvDivergence.Last()).Neg()
		// price mean reversion
		rev := fixedpoint.NewFromInt(1).Div(kline.Close)
		// alpha150 from GTJA's 191 paper
		a150 := kline.High.Add(kline.Low).Add(kline.Close).Div(three).Mul(kline.Volume)
		// momentum from WQ's 101 paper
		mom := fixedpoint.One.Sub(kline.Open.Div(kline.Close)).Mul(fixedpoint.NegOne)
		// opening gap
		ogap := kline.Open.Div(s.prevClose)

		log.Infof("corr: %f, rev: %f, a150: %f, mom: %f, ogap: %f", corr.Float64(), rev.Float64(), a150.Float64(), mom.Float64(), ogap.Float64())
		s.Alpha[0] = append(s.Alpha[0], corr.Float64())
		s.Alpha[1] = append(s.Alpha[1], rev.Float64())
		s.Alpha[2] = append(s.Alpha[2], a150.Float64())
		s.Alpha[3] = append(s.Alpha[3], mom.Float64())
		s.Alpha[4] = append(s.Alpha[4], ogap.Float64())

		// s.Alpha[5] = append(s.Alpha[4], 1.0) // constant

		ret := kline.Close.Sub(s.prevClose).Div(s.prevClose).Float64()
		s.Ret = append(s.Ret, ret)
		log.Infof("Current Return: %f", s.Ret[len(s.Ret)-1])

		// accumulate enough data for cross-sectional regression, not time-series regression
		s.T = 20
		if preCompute < int(s.T)+1 {
			preCompute++
		} else {
			s.ClosePosition(ctx, fixedpoint.One)
			s.tradeCollector.Process()
			// rolling regression for last 20 interval alphas
			r := new(regression.Regression)
			r.SetObserved("Return Rate Per Timeframe")
			r.SetVar(0, "Corr")
			r.SetVar(1, "Rev")
			r.SetVar(2, "A150")
			r.SetVar(3, "Mom")
			r.SetVar(4, "OGap")
			var rdp regression.DataPoints
			for i := 1; i <= int(s.T); i++ {
				// alphas[t-1], previous alphas, dot not take current alpha into account, will cause look-ahead bias
				as := []float64{s.Alpha[0][len(s.Alpha[0])-(i+2)], s.Alpha[1][len(s.Alpha[1])-(i+2)], s.Alpha[2][len(s.Alpha[2])-(i+2)], s.Alpha[3][len(s.Alpha[3])-(i+2)], s.Alpha[4][len(s.Alpha[4])-(i+2)]}
				// alphas[t], current return rate
				rt := s.Ret[len(s.Ret)-(i+1)]
				rdp = append(rdp, regression.DataPoint(rt, as))

			}
			r.Train(rdp...)
			r.Run()
			fmt.Printf("Regression formula:\n%v\n", r.Formula)
			// prediction := r.Coeff(0)*corr.Float64() + r.Coeff(1)*rev.Float64() + r.Coeff(2)*factorzoo.Float64() + r.Coeff(3)*mom.Float64() + r.Coeff(4)
			prediction, _ := r.Predict([]float64{corr.Float64(), rev.Float64(), a150.Float64(), mom.Float64(), ogap.Float64()})
			log.Infof("Predicted Return: %f", prediction)

			s.placeOrders(ctx, orderExecutor, fixedpoint.NewFromFloat(prediction))
			s.tradeCollector.Process()
		}

		s.prevClose = kline.Close

	})

	return nil
}
