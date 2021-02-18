package binance

import (
	"fmt"
	"strconv"
	"time"

	"github.com/adshao/go-binance/v2"
	"github.com/pkg/errors"

	"github.com/c9s/bbgo/pkg/datatype"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
)

func toGlobalTicker(stats *binance.PriceChangeStats) types.Ticker {
	return types.Ticker{
		Volume: util.MustParseFloat(stats.Volume),
		Last:   util.MustParseFloat(stats.LastPrice),
		Open:   util.MustParseFloat(stats.OpenPrice),
		High:   util.MustParseFloat(stats.HighPrice),
		Low:    util.MustParseFloat(stats.LowPrice),
		Buy:    util.MustParseFloat(stats.BidPrice),
		Sell:   util.MustParseFloat(stats.AskPrice),
		Time:   time.Unix(0, stats.CloseTime*int64(time.Millisecond)),
	}
}

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

func ToGlobalOrders(binanceOrders []*binance.Order) (orders []types.Order, err error) {
	for _, binanceOrder := range binanceOrders {
		order, err := ToGlobalOrder(binanceOrder, false)
		if err != nil {
			return orders, err
		}

		orders = append(orders, *order)
	}

	return orders, err
}

func ToGlobalOrder(binanceOrder *binance.Order, isMargin bool) (*types.Order, error) {
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
		Exchange:         types.ExchangeBinance.String(),
		IsWorking:        binanceOrder.IsWorking,
		OrderID:          uint64(binanceOrder.OrderID),
		Status:           toGlobalOrderStatus(binanceOrder.Status),
		ExecutedQuantity: util.MustParseFloat(binanceOrder.ExecutedQuantity),
		CreationTime:     datatype.Time(millisecondTime(binanceOrder.Time)),
		UpdateTime:       datatype.Time(millisecondTime(binanceOrder.UpdateTime)),
		IsMargin:         isMargin,
		IsIsolated:       binanceOrder.IsIsolated,
	}, nil
}

func millisecondTime(t int64) time.Time {
	return time.Unix(0, t*int64(time.Millisecond))
}

func ToGlobalTrade(t binance.TradeV3, isMargin bool) (*types.Trade, error) {
	// skip trade ID that is the same. however this should not happen
	var side types.SideType
	if t.IsBuyer {
		side = types.SideTypeBuy
	} else {
		side = types.SideTypeSell
	}

	price, err := strconv.ParseFloat(t.Price, 64)
	if err != nil {
		return nil, errors.Wrapf(err, "price parse error, price: %+v", t.Price)
	}

	quantity, err := strconv.ParseFloat(t.Quantity, 64)
	if err != nil {
		return nil, errors.Wrapf(err, "quantity parse error, quantity: %+v", t.Quantity)
	}

	var quoteQuantity = 0.0
	if len(t.QuoteQuantity) > 0 {
		quoteQuantity, err = strconv.ParseFloat(t.QuoteQuantity, 64)
		if err != nil {
			return nil, errors.Wrapf(err, "quote quantity parse error, quoteQuantity: %+v", t.QuoteQuantity)
		}
	}

	fee, err := strconv.ParseFloat(t.Commission, 64)
	if err != nil {
		return nil, errors.Wrapf(err, "commission parse error, commission: %+v", t.Commission)
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
		Time:          datatype.Time(millisecondTime(t.Time)),
		IsMargin:      isMargin,
		IsIsolated:    t.IsIsolated,
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

	case binance.OrderTypeLimit,
		binance.OrderTypeLimitMaker, binance.OrderTypeTakeProfitLimit:
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

// ConvertTrades converts the binance v3 trade into the global trade type
func ConvertTrades(remoteTrades []*binance.TradeV3) (trades []types.Trade, err error) {
	for _, t := range remoteTrades {
		trade, err := ToGlobalTrade(*t, false)
		if err != nil {
			return nil, errors.Wrapf(err, "binance v3 trade parse error, trade: %+v", *t)
		}

		trades = append(trades, *trade)
	}

	return trades, err
}
