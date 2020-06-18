package bbgo

import (
	"context"
	"github.com/adshao/go-binance"
	"github.com/sirupsen/logrus"
)

type BinanceExchange struct {
	Client *binance.Client
}

func (e *BinanceExchange) SubmitOrder(ctx context.Context, order Order) error {
	req := e.Client.NewCreateOrderService().
		Symbol(order.Symbol).
		Side(order.Side).
		Type(order.Type).
		Quantity(order.VolumeStr)

	if len(order.PriceStr) > 0 {
		req.Price(order.PriceStr)
	}
	if len(order.TimeInForce) > 0 {
		req.TimeInForce(order.TimeInForce)
	}

	retOrder, err := req.Do(ctx)
	logrus.Infof("order created: %+v", retOrder)
	return err
}

