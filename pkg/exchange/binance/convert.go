package binance

import (
	"fmt"
	"strconv"
	"time"

	"github.com/adshao/go-binance"

	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
)

func toLocalOrderType(orderType types.OrderType) (binance.OrderType, error) {
	switch orderType {
	case types.OrderTypeLimit:
		return binance.OrderTypeLimit, nil

	case types.OrderTypeStopLimit:
		return binance.OrderTypeStopLossLimit, nil

	case types.OrderTypeStopMarket:
		return binance.OrderTypeStopLoss, nil

	case types.OrderTypeMarket:
		return binance.OrderTypeMarket, nil
	}

	return "", fmt.Errorf("order type %s not supported", orderType)
}

func toGlobalOrder(binanceOrder *binance.Order) (*types.Order, error) {
	return &types.Order{
		SubmitOrder: types.SubmitOrder{
			ClientOrderID: binanceOrder.ClientOrderID,
			Symbol:        binanceOrder.Symbol,
			Side:          toGlobalSideType(binanceOrder.Side),
			Type:          toGlobalOrderType(binanceOrder.Type),
			Quantity:      util.MustParseFloat(binanceOrder.OrigQuantity),
			Price:         util.MustParseFloat(binanceOrder.Price),
			TimeInForce:   string(binanceOrder.TimeInForce),
		},
		IsWorking:        binanceOrder.IsWorking,
		OrderID:          uint64(binanceOrder.OrderID),
		Status:           toGlobalOrderStatus(binanceOrder.Status),
		ExecutedQuantity: util.MustParseFloat(binanceOrder.ExecutedQuantity),
	}, nil
}

func toGlobalTrade(t binance.TradeV3) (*types.Trade, error) {
	// skip trade ID that is the same. however this should not happen
	var side types.SideType
	if t.IsBuyer {
		side = types.SideTypeBuy
	} else {
		side = types.SideTypeSell
	}

	// trade time
	mts := time.Unix(0, t.Time*int64(time.Millisecond))

	price, err := strconv.ParseFloat(t.Price, 64)
	if err != nil {
		return nil, err
	}

	quantity, err := strconv.ParseFloat(t.Quantity, 64)
	if err != nil {
		return nil, err
	}

	quoteQuantity, err := strconv.ParseFloat(t.QuoteQuantity, 64)
	if err != nil {
		return nil, err
	}

	fee, err := strconv.ParseFloat(t.Commission, 64)
	if err != nil {
		return nil, err
	}

	return &types.Trade{
		ID:            t.ID,
		OrderID:       uint64(t.OrderID),
		Price:         price,
		Symbol:        t.Symbol,
		Exchange:      "binance",
		Quantity:      quantity,
		Side:          side,
		IsBuyer:       t.IsBuyer,
		IsMaker:       t.IsMaker,
		Fee:           fee,
		FeeCurrency:   t.CommissionAsset,
		QuoteQuantity: quoteQuantity,
		Time:          mts,
	}, nil
}

func toGlobalSideType(side binance.SideType) types.SideType {
	switch side {
	case binance.SideTypeBuy:
		return types.SideTypeBuy

	case binance.SideTypeSell:
		return types.SideTypeSell

	default:
		log.Errorf("unknown side type: %v", side)
		return ""
	}
}

func toGlobalOrderType(orderType binance.OrderType) types.OrderType {
	switch orderType {

	case binance.OrderTypeLimit:
		return types.OrderTypeLimit

	case binance.OrderTypeMarket:
		return types.OrderTypeMarket

	case binance.OrderTypeStopLossLimit:
		return types.OrderTypeStopLimit

	case binance.OrderTypeStopLoss:
		return types.OrderTypeStopMarket

	default:
		log.Errorf("unsupported order type: %v", orderType)
		return ""
	}
}

func toGlobalOrderStatus(orderStatus binance.OrderStatusType) types.OrderStatus {
	switch orderStatus {
	case binance.OrderStatusTypeNew:
		return types.OrderStatusNew

	case binance.OrderStatusTypeRejected:
		return types.OrderStatusRejected

	case binance.OrderStatusTypeCanceled:
		return types.OrderStatusCanceled

	case binance.OrderStatusTypePartiallyFilled:
		return types.OrderStatusPartiallyFilled

	case binance.OrderStatusTypeFilled:
		return types.OrderStatusFilled
	}

	return types.OrderStatus(orderStatus)
}
