package max

import (
	"fmt"
	"strings"
	"time"

	max "github.com/c9s/bbgo/pkg/exchange/max/maxapi"
	v3 "github.com/c9s/bbgo/pkg/exchange/max/maxapi/v3"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func toGlobalNetwork(network string) (out string) {
	out = network
	parts := strings.SplitN(network, "-", 2)
	if len(parts) > 0 {
		out = parts[0]
	}

	out = strings.ToUpper(out)
	return out
}

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

func toGlobalOrderStatus(
	orderState max.OrderState, executedVolume, remainingVolume fixedpoint.Value,
) types.OrderStatus {
	switch orderState {

	case max.OrderStateCancel:
		return types.OrderStatusCanceled

	case max.OrderStateFinalizing, max.OrderStateDone:
		if executedVolume.IsZero() {
			return types.OrderStatusCanceled
		} else if remainingVolume.IsZero() {
			return types.OrderStatusFilled
		}

		return types.OrderStatusFilled

	case max.OrderStateWait:
		if executedVolume.Sign() > 0 && remainingVolume.Sign() > 0 {
			return types.OrderStatusPartiallyFilled
		}

		return types.OrderStatusNew

	case max.OrderStateConvert:
		if executedVolume.Sign() > 0 && remainingVolume.Sign() > 0 {
			return types.OrderStatusPartiallyFilled
		}

		return types.OrderStatusNew

	case max.OrderStateFailed:
		return types.OrderStatusRejected

	}

	log.Errorf("can not convert MAX exchange order status, unknown order state: %q", orderState)
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
		return types.OrderTypeLimit

	case max.OrderTypePostOnly:
		return types.OrderTypeLimitMaker

	}

	log.Errorf("order convert error, unknown order type: %v", orderType)
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
	}

	return "", fmt.Errorf("order type %s not supported", orderType)
}

func toGlobalOrders(maxOrders []max.Order) (orders []types.Order, err error) {
	for _, localOrder := range maxOrders {
		o, err := toGlobalOrder(localOrder)
		if err != nil {
			log.WithError(err).Error("order convert error")
		} else {
			orders = append(orders, *o)
		}
	}

	return orders, err
}

func toGlobalOrder(maxOrder max.Order) (*types.Order, error) {
	executedVolume := maxOrder.ExecutedVolume
	remainingVolume := maxOrder.RemainingVolume
	isMargin := maxOrder.WalletType == max.WalletTypeMargin
	return &types.Order{
		SubmitOrder: types.SubmitOrder{
			ClientOrderID: maxOrder.ClientOID,
			Symbol:        toGlobalSymbol(maxOrder.Market),
			Side:          toGlobalSideType(maxOrder.Side),
			Type:          toGlobalOrderType(maxOrder.OrderType),
			Quantity:      maxOrder.Volume,
			Price:         maxOrder.Price,
			TimeInForce:   types.TimeInForceGTC, // MAX only supports GTC
			GroupID:       maxOrder.GroupID,
		},
		Exchange:         types.ExchangeMax,
		IsWorking:        maxOrder.State == max.OrderStateWait,
		OrderID:          maxOrder.ID,
		Status:           toGlobalOrderStatus(maxOrder.State, executedVolume, remainingVolume),
		OriginalStatus:   string(maxOrder.State),
		ExecutedQuantity: executedVolume,
		CreationTime:     types.Time(maxOrder.CreatedAt.Time()),
		UpdateTime:       types.Time(maxOrder.UpdatedAt.Time()),
		IsMargin:         isMargin,
		IsIsolated:       false, // isolated margin is not supported
	}, nil
}

func toGlobalTradeV3(t v3.Trade) ([]types.Trade, error) {
	var trades []types.Trade
	isMargin := t.WalletType == max.WalletTypeMargin
	side := toGlobalSideType(t.Side)

	fee := fixedpoint.Zero
	if t.Fee != nil {
		fee = *t.Fee
	}

	trade := types.Trade{
		ID:            t.ID,
		OrderID:       t.OrderID,
		Price:         t.Price,
		Symbol:        toGlobalSymbol(t.Market),
		Exchange:      types.ExchangeMax,
		Quantity:      t.Volume,
		Side:          side,
		IsBuyer:       t.IsBuyer(),
		IsMaker:       t.IsMaker(),
		Fee:           fee,
		FeeProcessing: t.Fee == nil,
		FeeCurrency:   toGlobalCurrency(t.FeeCurrency),
		FeeDiscounted: t.FeeDiscounted,
		QuoteQuantity: t.Funds,
		Time:          types.Time(t.CreatedAt),
		IsMargin:      isMargin,
		IsIsolated:    false,
		IsFutures:     false,
	}

	if t.Side == "self-trade" {
		trade.Side = types.SideTypeSell

		// create trade for bid
		bidTrade := trade
		bidTrade.Side = types.SideTypeBuy
		bidTrade.OrderID = t.SelfTradeBidOrderID
		bidTrade.Fee = t.SelfTradeBidFee
		bidTrade.FeeCurrency = toGlobalCurrency(t.SelfTradeBidFeeCurrency)
		bidTrade.FeeDiscounted = t.SelfTradeBidFeeDiscounted
		bidTrade.IsBuyer = !trade.IsBuyer
		bidTrade.IsMaker = !trade.IsMaker
		trades = append(trades, bidTrade)
	}

	trades = append(trades, trade)

	return trades, nil
}

func toGlobalDepositStatus(a max.DepositState) types.DepositStatus {
	switch a {

	case max.DepositStateSubmitting, max.DepositStateSubmitted, max.DepositStatePending, max.DepositStateChecking:
		return types.DepositPending

	case max.DepositStateRejected:
		return types.DepositRejected

	case max.DepositStateCancelled:
		return types.DepositCancelled

	case max.DepositStateAccepted:
		return types.DepositSuccess

	case max.DepositStateFailed: // v3 state
		return types.DepositRejected

	case max.DepositStateProcessing: // v3 states
		return types.DepositPending

	case max.DepositStateDone: // v3 states
		return types.DepositSuccess

	}

	// other states goes to this
	// max.DepositStateSuspect, max.DepositStateSuspended
	log.Errorf("unsupported deposit state %q from max exchange", a)
	return types.DepositStatus(a)
}

func convertWebSocketTrade(t max.TradeUpdate) (*types.Trade, error) {
	// skip trade ID that is the same. however this should not happen
	var side = toGlobalSideType(t.Side)

	return &types.Trade{
		ID:            t.ID,
		OrderID:       t.OrderID,
		Symbol:        toGlobalSymbol(t.Market),
		Exchange:      types.ExchangeMax,
		Price:         t.Price,
		Quantity:      t.Volume,
		Side:          side,
		IsBuyer:       side == types.SideTypeBuy,
		IsMaker:       t.Maker,
		Fee:           t.Fee,
		FeeCurrency:   toGlobalCurrency(t.FeeCurrency),
		FeeDiscounted: t.FeeDiscounted,
		QuoteQuantity: t.Funds,
		Time:          types.Time(t.Timestamp.Time()),
	}, nil
}

func convertWebSocketOrderUpdate(u max.OrderUpdate) (*types.Order, error) {
	timeInForce := types.TimeInForceGTC
	if u.OrderType == max.OrderTypeIOCLimit {
		timeInForce = types.TimeInForceIOC
	}

	return &types.Order{
		SubmitOrder: types.SubmitOrder{
			ClientOrderID: u.ClientOID,
			Symbol:        toGlobalSymbol(u.Market),
			Side:          toGlobalSideType(u.Side),
			Type:          toGlobalOrderType(u.OrderType),
			Quantity:      u.Volume,
			Price:         u.Price,
			StopPrice:     u.StopPrice,
			TimeInForce:   timeInForce, // MAX only supports GTC
			GroupID:       u.GroupID,
		},
		Exchange:         types.ExchangeMax,
		OrderID:          u.ID,
		Status:           toGlobalOrderStatus(u.State, u.ExecutedVolume, u.RemainingVolume),
		ExecutedQuantity: u.ExecutedVolume,
		CreationTime:     types.Time(time.Unix(0, u.CreatedAtMs*int64(time.Millisecond))),
		UpdateTime:       types.Time(time.Unix(0, u.UpdateTime*int64(time.Millisecond))),
	}, nil
}

func convertWithdrawStatusV3(status max.WithdrawStatus) types.WithdrawStatus {
	switch status {

	case max.WithdrawStatusPending:
		return types.WithdrawStatusSent

	case max.WithdrawStatusOK:
		return types.WithdrawStatusCompleted

	case max.WithdrawStatusFailed:
		return types.WithdrawStatusFailed

	case max.WithdrawStatusCancelled:
		return types.WithdrawStatusCancelled

	}

	return types.WithdrawStatus(status)
}

func convertWithdrawStatusV2(state max.WithdrawState) types.WithdrawStatus {
	switch state {

	case max.WithdrawStateSent, max.WithdrawStateSubmitting, max.WithdrawStatePending, "accepted", "approved":
		return types.WithdrawStatusSent

	case max.WithdrawStateProcessing, "delisted_processing", "kgi_manually_processing", "kgi_manually_confirmed", "sygna_verifying":
		return types.WithdrawStatusProcessing

	case max.WithdrawStateFailed, "kgi_possible_failed", "rejected", "suspect", "retryable":
		return types.WithdrawStatusFailed

	case max.WithdrawStateCanceled:
		return types.WithdrawStatusCancelled

	case "confirmed":
		// make it compatible with binance
		return types.WithdrawStatusCompleted

	default:
		return types.WithdrawStatus(state)
	}
}

func convertDepth(symbol string, depth *v3.Depth) (snapshot types.SliceOrderBook, finalUpdateID int64, err error) {
	snapshot.Symbol = toGlobalSymbol(symbol)
	snapshot.Time = time.Unix(depth.Timestamp, 0)
	snapshot.LastUpdateId = depth.LastUpdateId

	finalUpdateID = depth.LastUpdateId
	for _, entry := range depth.Bids {
		snapshot.Bids = append(snapshot.Bids, types.PriceVolume{Price: entry[0], Volume: entry[1]})
	}

	for _, entry := range depth.Asks {
		snapshot.Asks = append(snapshot.Asks, types.PriceVolume{Price: entry[0], Volume: entry[1]})
	}

	return snapshot, finalUpdateID, err
}

func toGlobalKLines(
	data []v3.KLineData, symbol string, period v3.Period, interval types.Interval, until *time.Time,
) ([]types.KLine, error) {
	var kLines []types.KLine
	for _, slice := range data {
		ts := int64(slice[0])
		startTime := time.Unix(ts, 0)
		endTime := startTime.Add(time.Duration(period)*time.Minute - time.Millisecond)
		isClosed := time.Now().Before(endTime)

		if until != nil && startTime.After(*until) {
			break
		}

		kLines = append(kLines, types.KLine{
			Symbol:    symbol,
			Interval:  interval,
			StartTime: types.Time(startTime),
			EndTime:   types.Time(endTime),
			Open:      fixedpoint.NewFromFloat(slice[1]),
			High:      fixedpoint.NewFromFloat(slice[2]),
			Low:       fixedpoint.NewFromFloat(slice[3]),
			Close:     fixedpoint.NewFromFloat(slice[4]),
			Volume:    fixedpoint.NewFromFloat(slice[5]),
			Closed:    isClosed,
		})
	}
	return kLines, nil

}
