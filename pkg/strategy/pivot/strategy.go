package pivot

import (
	"context"
	"fmt"
	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/sirupsen/logrus"
)

const ID = "pivot"

var fifteen = fixedpoint.NewFromInt(15)
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

	Position       *types.Position  `json:"position,omitempty"`
	StopLossRatio  fixedpoint.Value `json:"stopLossRatio"`
	CatBounceRatio fixedpoint.Value `json:"catBounceRatio"`
	ShadowTPRatio  fixedpoint.Value `json:"shadowTPRatio"`

	activeMakerOrders *bbgo.LocalActiveOrderBook
	orderStore        *bbgo.OrderStore
	tradeCollector    *bbgo.TradeCollector

	session *bbgo.ExchangeSession

	//pivotHigh *PIVOTHIGH
	pivot *Pivot
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	log.Infof("subscribe %s", s.Symbol)
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.Interval.String()})
	//session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: types.Interval1d.String()})

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
		Quantity: s.Quantity,
		Market:   s.Market,
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

	s.tradeCollector = bbgo.NewTradeCollector(s.Symbol, s.Position, s.orderStore)
	s.tradeCollector.BindStream(session.UserDataStream)

	iw := types.IntervalWindow{Window: 100, Interval: s.Interval}
	st, _ := session.MarketDataStore(s.Symbol)
	s.pivot = &Pivot{IntervalWindow: iw}
	s.pivot.Bind(st)

	session.UserDataStream.OnStart(func() {
		log.Infof("connected")
	})

	var lastLow fixedpoint.Value

	session.MarketDataStream.OnKLineClosed(func(kline types.KLine) {
		if kline.Symbol != s.Symbol || kline.Interval != s.Interval {
			return
		}
		if err := s.activeMakerOrders.GracefulCancel(ctx, s.session.Exchange); err != nil {
			log.WithError(err).Errorf("graceful cancel order error")
		}
		log.Info(s.pivot.LastLow())
		if s.pivot.LastLow() > 0. {

			lastLow = fixedpoint.NewFromFloat(s.pivot.LastLow())

		} else {
			lastLow = fixedpoint.Zero
			// SL || TP
			R := kline.Close.Div(s.Position.AverageCost)
			if R.Compare(fixedpoint.One.Add(s.StopLossRatio)) > 0 || R.Compare(fixedpoint.One.Sub(s.StopLossRatio.Mul(fifteen))) < 0 {
				if s.Position.GetBase().Compare(s.Quantity.Neg()) <= 0 {
					s.ClosePosition(ctx, fixedpoint.One)
					s.tradeCollector.Process()
				}
				// shadow TP
			} else if kline.GetLowerShadowHeight().Div(kline.Close).Compare(s.ShadowTPRatio) > 0 {
				s.ClosePosition(ctx, fixedpoint.One)
				s.tradeCollector.Process()
			}
		}
		if !lastLow.IsZero() {
			if s.Position.GetBase().Compare(s.Quantity.Neg()) > 0 {
				submitOrder := types.SubmitOrder{
					Symbol: s.Symbol,
					Side:   types.SideTypeSell,
					//Type:   types.OrderTypeMarket,
					Type: types.OrderTypeLimit,
					//Price: kline.Close,
					Price:    lastLow.Mul(fixedpoint.One.Add(s.CatBounceRatio)),
					Quantity: s.Quantity,
				}
				createdOrders, err := orderExecutor.SubmitOrders(ctx, submitOrder)
				if err != nil {
					log.WithError(err).Errorf("can not place orders")
				}
				s.orderStore.Add(createdOrders...)
				s.activeMakerOrders.Add(createdOrders...)
				s.tradeCollector.Process()
			}
		}
	})

	return nil
}
