package binance

import (
	"context"
	"fmt"

	"github.com/adshao/go-binance/v2"
	"github.com/c9s/bbgo/pkg/exchange/binance/binanceapi"
	"github.com/c9s/bbgo/pkg/types"
)

var _ types.ExchangeTradeService = (*Exchange)(nil)

func (e *Exchange) CancelReplace(ctx context.Context, cancelReplaceMode types.CancelReplaceModeType, o types.Order) (*types.Order, error) {
	if e.IsFutures || e.IsMargin {
		_ = cancelReplaceMode
		return types.CancelReplaceByCancelAndCreate(ctx, e, o)
	}
	if e.client2 == nil {
		return nil, types.NewOrderError(fmt.Errorf("binance cancel-replace client is not initialized"), o)
	}
	if err := orderLimiter.Wait(ctx); err != nil {
		log.WithError(err).Errorf("order rate limiter wait error")
		return nil, err
	}

	var req = e.client2.NewCancelReplaceSpotOrderRequest()
	req.Symbol(o.Symbol)
	req.Side(binance.SideType(o.Side))
	if o.OrderID > 0 {
		req.CancelOrderId(int(o.OrderID))
	} else if o.ClientOrderID != "" {
		req.CancelOrigClientOrderId(o.ClientOrderID)
	} else {
		return nil, types.NewOrderError(fmt.Errorf("cannot cancel %s order without order ID or client order ID", o.Symbol), o)
	}
	req.CancelReplaceMode(binanceapi.CancelReplaceModeType(cancelReplaceMode))
	if o.ClientOrderID != "" {
		req.NewClientOrderId(o.ClientOrderID)
	}
	if len(o.TimeInForce) > 0 {
		// TODO: check the TimeInForce value
		req.TimeInForce(string(binance.TimeInForceType(o.TimeInForce)))
	} else {
		switch o.Type {
		case types.OrderTypeLimit, types.OrderTypeStopLimit:
			req.TimeInForce(string(binance.TimeInForceTypeGTC))
		}
	}
	if o.Market.Symbol != "" {
		req.Quantity(o.Market.FormatQuantity(o.Quantity))
	} else {
		req.Quantity(o.Quantity.FormatString(8))
	}

	switch o.Type {
	case types.OrderTypeStopLimit, types.OrderTypeLimit, types.OrderTypeLimitMaker:
		if o.Market.Symbol != "" {
			req.Price(o.Market.FormatPrice(o.Price))
		} else {
			// TODO: report error
			req.Price(o.Price.FormatString(8))
		}
	}
	switch o.Type {
	case types.OrderTypeStopLimit, types.OrderTypeStopMarket:
		if o.Market.Symbol != "" {
			req.StopPrice(o.Market.FormatPrice(o.StopPrice))
		} else {
			// TODO report error
			req.StopPrice(o.StopPrice.FormatString(8))
		}
	}
	req.NewOrderRespType(binanceapi.Full)

	resp, err := req.Do(ctx)
	if err != nil {
		return nil, err
	}
	if resp == nil || resp.Data == nil {
		return nil, types.NewOrderError(fmt.Errorf("binance cancel-replace returned no result data"), o)
	}
	if resp.Data.CancelResult != "SUCCESS" ||
		resp.Data.NewOrderResult != "SUCCESS" ||
		resp.Data.NewOrderResponse == nil {
		return nil, types.NewOrderError(fmt.Errorf(
			"binance cancel-replace partial result: cancelResult=%s newOrderResult=%s newOrderResponsePresent=%t",
			resp.Data.CancelResult, resp.Data.NewOrderResult, resp.Data.NewOrderResponse != nil), o)
	}

	newOrder, err := toGlobalOrder(resp.Data.NewOrderResponse, e.IsMargin)
	if err != nil {
		return nil, err
	}
	return newOrder, nil
}
