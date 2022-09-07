package audacitymaker

import (
	"context"
	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"

	"gonum.org/v1/gonum/stat"
)

type PerTrade struct {
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

func (s *PerTrade) Bind(session *bbgo.ExchangeSession, orderExecutor *bbgo.GeneralOrderExecutor) {
	s.session = session
	s.orderExecutor = orderExecutor

	position := orderExecutor.Position()
	symbol := position.Symbol
	// ger best bid/ask, not used yet
	s.StreamBook = types.NewStreamBook(symbol)
	s.StreamBook.BindStream(session.MarketDataStream)

	// use queue to do time-series rolling
	buyTradeSize := types.NewQueue(200)
	sellTradeSize := types.NewQueue(200)
	buyTradesNumber := types.NewQueue(200)
	sellTradesNumber := types.NewQueue(200)
	// [WIP] Order Aggressiveness refers to the percentage of orders that are submitted at market prices, as opposed to limit prices.

	// Order flow is the difference between buyer-initiated and seller-initiated trading volume or number of trades.
	var orderFlowSize floats.Slice
	var orderFlowNumber floats.Slice

	var orderFlowSizeMinMax floats.Slice
	var orderFlowNumberMinMax floats.Slice

	threshold := 3.

	session.MarketDataStream.OnMarketTrade(func(trade types.Trade) {

		//log.Infof("%s trade @ %f", trade.Side, trade.Price.Float64())

		ctx := context.Background()

		if trade.Side == types.SideTypeBuy {
			// accumulating trading volume from buyer
			buyTradeSize.Update(trade.Quantity.Float64())
			sellTradeSize.Update(0)
			// counting trades of number from seller
			buyTradesNumber.Update(1)
			sellTradesNumber.Update(0)

		} else if trade.Side == types.SideTypeSell {
			// accumulating trading volume from buyer
			buyTradeSize.Update(0)
			sellTradeSize.Update(trade.Quantity.Float64())
			// counting trades of number from seller
			buyTradesNumber.Update(0)
			sellTradesNumber.Update(1)
		}

		//canceled := s.orderExecutor.GracefulCancel(ctx)
		//if canceled != nil {
		//	_ = s.orderExecutor.GracefulCancel(ctx)
		//}

		sizeFraction := buyTradeSize.Sum() / sellTradeSize.Sum()
		numberFraction := buyTradesNumber.Sum() / sellTradesNumber.Sum()
		orderFlowSize.Push(sizeFraction)
		if orderFlowSize.Length() > 100 {
			// min-max scaling
			ofsMax := orderFlowSize.Tail(100).Max()
			ofsMin := orderFlowSize.Tail(100).Min()
			ofsMinMax := (orderFlowSize.Last() - ofsMin) / (ofsMax - ofsMin)
			// preserves temporal dependency via polar encoded angles
			orderFlowSizeMinMax.Push(ofsMinMax)
		}

		orderFlowNumber.Push(numberFraction)
		if orderFlowNumber.Length() > 100 {
			// min-max scaling
			ofnMax := orderFlowNumber.Tail(100).Max()
			ofnMin := orderFlowNumber.Tail(100).Min()
			ofnMinMax := (orderFlowNumber.Last() - ofnMin) / (ofnMax - ofnMin)
			// preserves temporal dependency via polar encoded angles
			orderFlowNumberMinMax.Push(ofnMinMax)
		}

		if orderFlowSizeMinMax.Length() > 100 && orderFlowNumberMinMax.Length() > 100 {
			bid, ask, _ := s.StreamBook.BestBidAndAsk()
			if outlier(orderFlowSizeMinMax.Tail(100), threshold) > 0 && outlier(orderFlowNumberMinMax.Tail(100), threshold) > 0 {
				_ = s.orderExecutor.GracefulCancel(ctx)
				log.Infof("long!!")
				//_ = s.placeTrade(ctx, types.SideTypeBuy, s.Quantity, symbol)
				_ = s.placeOrder(ctx, types.SideTypeBuy, s.Quantity, bid.Price, symbol)
				//_ = s.placeOrder(ctx, types.SideTypeSell, s.Quantity, ask.Price.Mul(fixedpoint.NewFromFloat(1.0005)), symbol)
			} else if outlier(orderFlowSizeMinMax.Tail(100), threshold) < 0 && outlier(orderFlowNumberMinMax.Tail(100), threshold) < 0 {
				_ = s.orderExecutor.GracefulCancel(ctx)
				log.Infof("short!!")
				//_ = s.placeTrade(ctx, types.SideTypeSell, s.Quantity, symbol)
				_ = s.placeOrder(ctx, types.SideTypeSell, s.Quantity, ask.Price, symbol)
				//_ = s.placeOrder(ctx, types.SideTypeBuy, s.Quantity, bid.Price.Mul(fixedpoint.NewFromFloat(0.9995)), symbol)
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

func (s *PerTrade) placeOrder(ctx context.Context, side types.SideType, quantity fixedpoint.Value, price fixedpoint.Value, symbol string) error {
	market, _ := s.session.Market(symbol)
	_, err := s.orderExecutor.SubmitOrders(ctx, types.SubmitOrder{
		Symbol:   symbol,
		Market:   market,
		Side:     side,
		Type:     types.OrderTypeLimitMaker,
		Quantity: quantity,
		Price:    price,
		Tag:      "audacity-limit",
	})
	return err
}

func (s *PerTrade) placeTrade(ctx context.Context, side types.SideType, quantity fixedpoint.Value, symbol string) error {
	market, _ := s.session.Market(symbol)
	_, err := s.orderExecutor.SubmitOrders(ctx, types.SubmitOrder{
		Symbol:   symbol,
		Market:   market,
		Side:     side,
		Type:     types.OrderTypeMarket,
		Quantity: quantity,
		Tag:      "audacity-market",
	})
	return err
}

func outlier(fs floats.Slice, multiplier float64) int {
	stddev := stat.StdDev(fs, nil)
	if fs.Last() > fs.Mean()+multiplier*stddev {
		return 1
	} else if fs.Last() < fs.Mean()-multiplier*stddev {
		return -1
	}
	return 0
}
