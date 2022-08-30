package ktrade

import (
	"context"
	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type Minute struct {
	Symbol string
	Market types.Market `json:"-"`
	types.IntervalWindow

	// MarketOrder is the option to enable market order short.
	MarketOrder bool `json:"marketOrder"`

	Quantity fixedpoint.Value `json:"quantity"`

	orderExecutor *bbgo.GeneralOrderExecutor
	session       *bbgo.ExchangeSession
	activeOrders  *bbgo.ActiveOrderBook

	StreamBook *types.StreamOrderBook

	midPrice fixedpoint.Value

	bbgo.QuantityOrAmount
}

func (s *Minute) updateQuote(ctx context.Context, symbol string) {
	//bestBid, bestAsk, _ := s.StreamBook.BestBidAndAsk()

	//s.midPrice = bestBid.Price.Add(bestAsk.Price).Div(fixedpoint.NewFromInt(2))
	//log.Info(s.midPrice)
}

func (s *Minute) Bind(session *bbgo.ExchangeSession, orderExecutor *bbgo.GeneralOrderExecutor) {
	s.session = session
	s.orderExecutor = orderExecutor

	position := orderExecutor.Position()
	symbol := position.Symbol
	s.StreamBook = types.NewStreamBook(symbol)
	s.StreamBook.BindStream(session.MarketDataStream)

	//store, _ := session.MarketDataStore(symbol)

	session.MarketDataStream.OnMarketTrade(func(trade types.Trade) {

		log.Infof("%s trade @ %f", trade.Side, trade.Price.Float64())
		bestAsk, _ := s.StreamBook.BestAsk()
		bestBid, _ := s.StreamBook.BestBid()
		s.midPrice = bestBid.Price.Add(bestAsk.Price).Div(fixedpoint.NewFromInt(2))

		if trade.Side == types.SideTypeBuy && trade.Price.Compare(s.midPrice) > 0 {
			_ = s.orderExecutor.GracefulCancel(context.Background())
			// update ask price
			newAskPrice := s.midPrice.Mul(fixedpoint.NewFromFloat(0.25)).Add(trade.Price.Mul(fixedpoint.NewFromFloat(0.75)))
			log.Infof("short @ %f", newAskPrice.Float64())
			if trade.Price.Compare(newAskPrice) > 0 {
				s.placeOrder(context.Background(), types.SideTypeSell, s.Quantity, s.midPrice.Mul(fixedpoint.NewFromFloat(1.001)), symbol)
			} else {
				log.Infof("new")
				s.placeOrder(context.Background(), types.SideTypeSell, s.Quantity, newAskPrice.Round(2, 1), symbol)
			}

		} else if trade.Side == types.SideTypeSell && trade.Price.Compare(s.midPrice) < 0 {
			_ = s.orderExecutor.GracefulCancel(context.Background())
			// update bid price
			newBidPrice := s.midPrice.Mul(fixedpoint.NewFromFloat(0.25)).Add(trade.Price.Mul(fixedpoint.NewFromFloat(0.75)))
			log.Infof("long @ %f", newBidPrice.Float64())
			if trade.Price.Compare(newBidPrice) < 0 {
				s.placeOrder(context.Background(), types.SideTypeBuy, s.Quantity, s.midPrice.Mul(fixedpoint.NewFromFloat(0.999)), symbol)
			} else {
				log.Infof("new")
				s.placeOrder(context.Background(), types.SideTypeBuy, s.Quantity, newBidPrice.Round(2, 1), symbol)
			}
		}

	})

	session.MarketDataStream.OnKLineClosed(types.KLineWith(symbol, s.Interval, func(kline types.KLine) {

		log.Info(kline.NumberOfTrades)

	}))

	//go func() {
	//	quoteTicker := time.NewTicker(util.MillisecondsJitter(time.Millisecond*10, 200))
	//	defer quoteTicker.Stop()
	//
	//	for {
	//		select {
	//		case <-quoteTicker.C:
	//			s.updateQuote(context.Background(), symbol)
	//		}
	//	}
	//}()

	if !bbgo.IsBackTesting {
		session.MarketDataStream.OnMarketTrade(func(trade types.Trade) {
		})
	}
}

func (s *Minute) placeOrder(ctx context.Context, side types.SideType, quantity fixedpoint.Value, price fixedpoint.Value, symbol string) {
	market, _ := s.session.Market(symbol)
	_, err := s.orderExecutor.SubmitOrders(ctx, types.SubmitOrder{
		Symbol:   symbol,
		Market:   market,
		Side:     side,
		Type:     types.OrderTypeLimitMaker,
		Quantity: quantity,
		Price:    price,
		//TimeInForce: types.TimeInForceGTC,
		Tag: "ktrade",
	})
	if err != nil {
		log.WithError(err).Errorf("can not place order")
	}
}
