package grpc

import (
	"fmt"
	"strconv"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/pb"
	"github.com/c9s/bbgo/pkg/types"
)

func toSubscriptions(sub *pb.Subscription) (types.Subscription, error) {
	switch sub.Channel {
	case pb.Channel_TRADE:
		return types.Subscription{
			Symbol:  sub.Symbol,
			Channel: types.MarketTradeChannel,
		}, nil

	case pb.Channel_BOOK:
		return types.Subscription{
			Symbol:  sub.Symbol,
			Channel: types.BookChannel,
			Options: types.SubscribeOptions{
				Depth: types.Depth(sub.Depth),
			},
		}, nil

	case pb.Channel_KLINE:
		return types.Subscription{
			Symbol:  sub.Symbol,
			Channel: types.KLineChannel,
			Options: types.SubscribeOptions{
				Interval: sub.Interval,
			},
		}, nil
	}

	return types.Subscription{}, fmt.Errorf("unsupported subscription channel: %s", sub.Channel)
}

func transPriceVolume(srcPvs types.PriceVolumeSlice) (pvs []*pb.PriceVolume) {
	for _, srcPv := range srcPvs {
		pvs = append(pvs, &pb.PriceVolume{
			Price:  srcPv.Price.String(),
			Volume: srcPv.Volume.String(),
		})
	}
	return pvs
}

func transBook(session *bbgo.ExchangeSession, book types.SliceOrderBook, event pb.Event) *pb.MarketData {
	return &pb.MarketData{
		Session:  session.Name,
		Exchange: session.ExchangeName.String(),
		Symbol:   book.Symbol,
		Channel:  pb.Channel_BOOK,
		Event:    event,
		Depth: &pb.Depth{
			Exchange: session.ExchangeName.String(),
			Symbol:   book.Symbol,
			Asks:     transPriceVolume(book.Asks),
			Bids:     transPriceVolume(book.Bids),
		},
	}
}

func transMarketTrade(session *bbgo.ExchangeSession, marketTrade types.Trade) *pb.MarketData {
	return &pb.MarketData{
		Session:  session.Name,
		Exchange: session.ExchangeName.String(),
		Symbol:   marketTrade.Symbol,
		Channel:  pb.Channel_TRADE,
		Event:    pb.Event_UPDATE,
		Trades: []*pb.Trade{
			{
				Exchange:    marketTrade.Exchange.String(),
				Symbol:      marketTrade.Symbol,
				Id:          strconv.FormatUint(marketTrade.ID, 10),
				Price:       marketTrade.Price.String(),
				Quantity:    marketTrade.Quantity.String(),
				CreatedAt:   marketTrade.Time.UnixMilli(),
				Side:        transSide(marketTrade.Side),
				FeeCurrency: marketTrade.FeeCurrency,
				Fee:         marketTrade.Fee.String(),
				Maker:       marketTrade.IsMaker,
			},
		},
	}
}

func transSide(side types.SideType) pb.Side {
	switch side {
	case types.SideTypeBuy:
		return pb.Side_BUY
	case types.SideTypeSell:
		return pb.Side_SELL
	}

	return pb.Side_SELL
}

func transOrderType(orderType types.OrderType) pb.OrderType {
	switch orderType {
	case types.OrderTypeLimit:
		return pb.OrderType_LIMIT
	case types.OrderTypeMarket:
		return pb.OrderType_MARKET
	case types.OrderTypeStopLimit:
		return pb.OrderType_STOP_LIMIT
	case types.OrderTypeStopMarket:
		return pb.OrderType_STOP_MARKET
	}

	return pb.OrderType_LIMIT
}

func transOrder(session *bbgo.ExchangeSession, order types.Order) *pb.Order {
	return &pb.Order{
		Exchange:       order.Exchange.String(),
		Symbol:         order.Symbol,
		Id:             strconv.FormatUint(order.OrderID, 10),
		Side:           transSide(order.Side),
		OrderType:      transOrderType(order.Type),
		Price:          order.Price.String(),
		StopPrice:      order.StopPrice.String(),
		Status:         string(order.Status),
		CreatedAt:      order.CreationTime.UnixMilli(),
		Quantity:       order.Quantity.String(),
		ExecutedQuantity: order.ExecutedQuantity.String(),
		ClientOrderId:  order.ClientOrderID,
		GroupId:        0,
	}
}

func transKLine(session *bbgo.ExchangeSession, kline types.KLine) *pb.KLine {
	return &pb.KLine{
		Session:     session.Name,
		Exchange:    kline.Exchange.String(),
		Symbol:      kline.Symbol,
		Open:        kline.Open.String(),
		High:        kline.High.String(),
		Low:         kline.Low.String(),
		Close:       kline.Close.String(),
		Volume:      kline.Volume.String(),
		QuoteVolume: kline.QuoteVolume.String(),
		StartTime:   kline.StartTime.UnixMilli(),
		EndTime:     kline.StartTime.UnixMilli(),
		Closed:      kline.Closed,
	}
}

func transKLineResponse(session *bbgo.ExchangeSession, kline types.KLine) *pb.MarketData {
	return &pb.MarketData{
		Session:      session.Name,
		Exchange:     kline.Exchange.String(),
		Symbol:       kline.Symbol,
		Channel:      pb.Channel_KLINE,
		Event:        pb.Event_UPDATE,
		Kline:        transKLine(session, kline),
		SubscribedAt: 0,
	}
}
