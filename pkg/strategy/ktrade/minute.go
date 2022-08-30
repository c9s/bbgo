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
		bestBid, bestAsk, hasPrice := s.StreamBook.BestBidAndAsk()
		if !hasPrice {
			return
		}
		s.midPrice = bestBid.Price.Add(bestAsk.Price).Div(fixedpoint.NewFromInt(2))
		ctx := context.Background()

		if trade.Side == types.SideTypeBuy && trade.Price.Div(bestAsk.Price).Compare(fixedpoint.NewFromFloat(1.00005)) > 0 {
			canceled := s.orderExecutor.GracefulCancel(ctx)
			if canceled != nil {
				_ = s.orderExecutor.GracefulCancel(ctx)
			}
			// update ask price
			//bestAsk, _ = s.StreamBook.BestAsk()
			newAskPrice := bestAsk.Price.Mul(fixedpoint.NewFromFloat(1.001)).Mul(fixedpoint.NewFromFloat(0.25)).Add(trade.Price.Mul(fixedpoint.NewFromFloat(0.75)))
			log.Infof("short @ %f", newAskPrice.Float64())
			err := s.placeOrder(context.Background(), types.SideTypeSell, s.Quantity, newAskPrice.Round(2, 0), symbol)
			if err != nil {
				newAskPrice = bestAsk.Price.Mul(fixedpoint.NewFromFloat(1.002)).Mul(fixedpoint.NewFromFloat(0.25)).Add(trade.Price.Mul(fixedpoint.NewFromFloat(0.75)))
				log.Infof("short again @ %f", newAskPrice.Float64())
				_ = s.placeOrder(context.Background(), types.SideTypeSell, s.Quantity, newAskPrice.Round(2, 0), symbol)
			}
		} else if trade.Side == types.SideTypeSell && trade.Price.Div(bestBid.Price).Compare(fixedpoint.NewFromFloat(0.99995)) < 0 {
			canceled := s.orderExecutor.GracefulCancel(ctx)
			if canceled != nil {
				_ = s.orderExecutor.GracefulCancel(ctx)
			}
			// update bid price
			//bestBid, _ = s.StreamBook.BestBid()
			newBidPrice := bestBid.Price.Mul(fixedpoint.NewFromFloat(0.999)).Mul(fixedpoint.NewFromFloat(0.25)).Add(trade.Price.Mul(fixedpoint.NewFromFloat(0.75)))
			log.Infof("long @ %f", newBidPrice.Float64())
			err := s.placeOrder(context.Background(), types.SideTypeBuy, s.Quantity, newBidPrice.Round(2, 2), symbol)
			if err != nil {
				newBidPrice = bestBid.Price.Mul(fixedpoint.NewFromFloat(0.998)).Mul(fixedpoint.NewFromFloat(0.25)).Add(trade.Price.Mul(fixedpoint.NewFromFloat(0.75)))
				log.Infof("long again @ %f", newBidPrice.Float64())
				_ = s.placeOrder(context.Background(), types.SideTypeBuy, s.Quantity, newBidPrice.Round(2, 2), symbol)
			}
		}

	})

	session.MarketDataStream.OnKLineClosed(types.KLineWith(symbol, s.Interval, func(kline types.KLine) {

		log.Info(kline.NumberOfTrades)

	}))

	if !bbgo.IsBackTesting {
		session.MarketDataStream.OnMarketTrade(func(trade types.Trade) {
		})
	}
}

func (s *Minute) placeOrder(ctx context.Context, side types.SideType, quantity fixedpoint.Value, price fixedpoint.Value, symbol string) error {
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
	return err
}
