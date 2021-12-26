package max

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/c9s/bbgo/pkg/exchange/max/maxapi"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
)

func toGlobalCurrency(currency string) string {
	return strings.ToUpper(currency)
}

func toLocalCurrency(currency string) string {
	return strings.ToLower(currency)
}

func toLocalSymbol(symbol string) string {
	return strings.ToLower(symbol)
}

func toGlobalSymbol(symbol string) string {
	return strings.ToUpper(symbol)
}

func toLocalSideType(side types.SideType) string {
	return strings.ToLower(string(side))
}

func toGlobalSideType(v string) types.SideType {
	switch strings.ToLower(v) {
	case "bid", "buy":
		return types.SideTypeBuy

	case "ask", "sell":
		return types.SideTypeSell

	case "self-trade":
		return types.SideTypeSelf

	}

	return types.SideType(v)
}

func toGlobalRewards(maxRewards []max.Reward) ([]types.Reward, error) {
	// convert to global reward
	var rewards []types.Reward
	for _, r := range maxRewards {
		// ignore "accepted"
		if r.State != "done" {
			continue
		}

		reward, err := r.Reward()
		if err != nil {
			return nil, err
		}

		rewards = append(rewards, *reward)
	}

	return rewards, nil
}

func toGlobalOrderStatus(orderState max.OrderState, executedVolume, remainingVolume fixedpoint.Value) types.OrderStatus {
	switch orderState {

	case max.OrderStateFinalizing, max.OrderStateDone, max.OrderStateCancel:
		if executedVolume > 0 && remainingVolume > 0 {
			return types.OrderStatusPartiallyFilled
		} else if remainingVolume == 0 {
			return types.OrderStatusFilled
		} else if executedVolume == 0 {
			return types.OrderStatusCanceled
		}

		return types.OrderStatusFilled

	case max.OrderStateWait:
		if executedVolume > 0 && remainingVolume > 0 {
			return types.OrderStatusPartiallyFilled
		}

		return types.OrderStatusNew

	case max.OrderStateConvert:
		if executedVolume > 0 && remainingVolume > 0 {
			return types.OrderStatusPartiallyFilled
		}

		return types.OrderStatusNew

	case max.OrderStateFailed:
		return types.OrderStatusRejected

	}

	logger.Errorf("unknown order status: %v", orderState)
	return types.OrderStatus(orderState)
}

func toGlobalOrderType(orderType max.OrderType) types.OrderType {
	switch orderType {
	case max.OrderTypeLimit:
		return types.OrderTypeLimit

	case max.OrderTypeMarket:
		return types.OrderTypeMarket

	case max.OrderTypeStopLimit:
		return types.OrderTypeStopLimit

	case max.OrderTypeStopMarket:
		return types.OrderTypeStopMarket

	case max.OrderTypeIOCLimit:
		return types.OrderTypeIOCLimit

	case max.OrderTypePostOnly:
		return types.OrderTypeLimitMaker

	}

	logger.Errorf("order convert error, unknown order type: %v", orderType)
	return types.OrderType(orderType)
}

func toLocalOrderType(orderType types.OrderType) (max.OrderType, error) {
	switch orderType {

	case types.OrderTypeStopLimit:
		return max.OrderTypeStopLimit, nil

	case types.OrderTypeStopMarket:
		return max.OrderTypeStopMarket, nil

	case types.OrderTypeLimitMaker:
		return max.OrderTypePostOnly, nil

	case types.OrderTypeLimit:
		return max.OrderTypeLimit, nil

	case types.OrderTypeMarket:
		return max.OrderTypeMarket, nil

	case types.OrderTypeIOCLimit:
		return max.OrderTypeIOCLimit, nil
	}

	return "", fmt.Errorf("order type %s not supported", orderType)
}

func toGlobalOrders(maxOrders []max.Order) (orders []types.Order, err error) {
	for _, localOrder := range maxOrders {
		o, err := toGlobalOrder(localOrder)
		if err != nil {
			log.WithError(err).Error("order convert error")
		}

		orders = append(orders, *o)
	}

	return orders, err
}

func toGlobalOrder(maxOrder max.Order) (*types.Order, error) {
	executedVolume, err := fixedpoint.NewFromString(maxOrder.ExecutedVolume)
	if err != nil {
		return nil, errors.Wrapf(err, "parse executed_volume failed: %+v", maxOrder)
	}

	remainingVolume, err := fixedpoint.NewFromString(maxOrder.RemainingVolume)
	if err != nil {
		return nil, errors.Wrapf(err, "parse remaining volume failed: %+v", maxOrder)
	}

	return &types.Order{
		SubmitOrder: types.SubmitOrder{
			ClientOrderID: maxOrder.ClientOID,
			Symbol:        toGlobalSymbol(maxOrder.Market),
			Side:          toGlobalSideType(maxOrder.Side),
			Type:          toGlobalOrderType(maxOrder.OrderType),
			Quantity:      util.MustParseFloat(maxOrder.Volume),
			Price:         util.MustParseFloat(maxOrder.Price),
			TimeInForce:   "GTC", // MAX only supports GTC
			GroupID:       maxOrder.GroupID,
		},
		Exchange:         types.ExchangeMax,
		IsWorking:        maxOrder.State == "wait",
		OrderID:          maxOrder.ID,
		Status:           toGlobalOrderStatus(maxOrder.State, executedVolume, remainingVolume),
		ExecutedQuantity: executedVolume.Float64(),
		CreationTime:     types.Time(maxOrder.CreatedAt),
		UpdateTime:       types.Time(maxOrder.CreatedAt),
	}, nil
}

func toGlobalTrade(t max.Trade) (*types.Trade, error) {
	// skip trade ID that is the same. however this should not happen
	var side = toGlobalSideType(t.Side)

	// trade time
	mts := time.Unix(0, t.CreatedAtMilliSeconds*int64(time.Millisecond))

	price, err := strconv.ParseFloat(t.Price, 64)
	if err != nil {
		return nil, err
	}

	quantity, err := strconv.ParseFloat(t.Volume, 64)
	if err != nil {
		return nil, err
	}

	quoteQuantity, err := strconv.ParseFloat(t.Funds, 64)
	if err != nil {
		return nil, err
	}

	fee, err := strconv.ParseFloat(t.Fee, 64)
	if err != nil {
		return nil, err
	}

	return &types.Trade{
		ID:            t.ID,
		OrderID:       t.OrderID,
		Price:         price,
		Symbol:        toGlobalSymbol(t.Market),
		Exchange:      "max",
		Quantity:      quantity,
		Side:          side,
		IsBuyer:       t.IsBuyer(),
		IsMaker:       t.IsMaker(),
		Fee:           fee,
		FeeCurrency:   toGlobalCurrency(t.FeeCurrency),
		QuoteQuantity: quoteQuantity,
		Time:          types.Time(mts),
	}, nil
}

func toGlobalDepositStatus(a string) types.DepositStatus {
	switch a {
	case "submitting", "submitted", "checking":
		return types.DepositPending

	case "accepted":
		return types.DepositSuccess

	case "rejected":
		return types.DepositRejected

	case "canceled":
		return types.DepositCancelled

	case "suspect", "refunded":

	}

	return types.DepositStatus(a)
}
