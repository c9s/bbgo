package backpack

import (
	"fmt"
	"strconv"
	"time"

	"github.com/c9s/bbgo/pkg/exchange/backpack/backpackapi"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
)

func toGlobalSide(side backpackapi.Side) types.SideType {
	switch side {
	case backpackapi.SideBid:
		return types.SideTypeBuy
	case backpackapi.SideAsk:
		return types.SideTypeSell
	}

	return types.SideType(side)
}

func toLocalSide(side types.SideType) (backpackapi.Side, error) {
	switch side {
	case types.SideTypeBuy:
		return backpackapi.SideBid, nil
	case types.SideTypeSell:
		return backpackapi.SideAsk, nil
	}

	return "", fmt.Errorf("unsupported side type: %s", side)
}

func toGlobalOrderType(orderType backpackapi.OrderType, postOnly bool) types.OrderType {
	switch orderType {
	case backpackapi.OrderTypeMarket:
		return types.OrderTypeMarket

	case backpackapi.OrderTypeLimit:
		// Backpack carries post-only as its own flag rather than as an order type
		if postOnly {
			return types.OrderTypeLimitMaker
		}

		return types.OrderTypeLimit
	}

	return types.OrderType(orderType)
}

// toLocalOrderType converts a bbgo order type into the Backpack order type and reports whether
// the order has to be submitted with the postOnly flag.
func toLocalOrderType(orderType types.OrderType) (localType backpackapi.OrderType, postOnly bool, err error) {
	switch orderType {
	case types.OrderTypeMarket:
		return backpackapi.OrderTypeMarket, false, nil

	case types.OrderTypeLimit:
		return backpackapi.OrderTypeLimit, false, nil

	case types.OrderTypeLimitMaker:
		return backpackapi.OrderTypeLimit, true, nil
	}

	return "", false, fmt.Errorf("unsupported order type: %s", orderType)
}

func toGlobalOrderStatus(status backpackapi.OrderStatus) (types.OrderStatus, error) {
	switch status {
	case backpackapi.OrderStatusNew:
		return types.OrderStatusNew, nil
	case backpackapi.OrderStatusPartiallyFilled:
		return types.OrderStatusPartiallyFilled, nil
	case backpackapi.OrderStatusFilled:
		return types.OrderStatusFilled, nil
	case backpackapi.OrderStatusCancelled:
		return types.OrderStatusCanceled, nil
	case backpackapi.OrderStatusExpired:
		return types.OrderStatusExpired, nil
	case backpackapi.OrderStatusTriggerPending:
		return types.OrderStatusTriggering, nil
	case backpackapi.OrderStatusTriggerFailed:
		return types.OrderStatusRejected, nil
	}

	return "", fmt.Errorf("unknown or unsupported backpack order status: %s", status)
}

func toGlobalTimeInForce(tif backpackapi.TimeInForce) types.TimeInForce {
	switch tif {
	case backpackapi.TimeInForceGTC:
		return types.TimeInForceGTC
	case backpackapi.TimeInForceIOC:
		return types.TimeInForceIOC
	case backpackapi.TimeInForceFOK:
		return types.TimeInForceFOK
	}

	return types.TimeInForce(tif)
}

func toLocalTimeInForce(tif types.TimeInForce) (backpackapi.TimeInForce, error) {
	switch tif {
	case types.TimeInForceGTC:
		return backpackapi.TimeInForceGTC, nil
	case types.TimeInForceIOC:
		return backpackapi.TimeInForceIOC, nil
	case types.TimeInForceFOK:
		return backpackapi.TimeInForceFOK, nil
	}

	return "", fmt.Errorf("unsupported time in force: %s", tif)
}

func toLocalInterval(interval types.Interval) (backpackapi.KlineInterval, error) {
	localInterval, ok := localIntervals[interval]
	if !ok {
		return "", fmt.Errorf("interval %s is not supported by backpack", interval)
	}

	return localInterval, nil
}

// toGlobalOrderID converts a Backpack order id into the uint64 that types.Order.OrderID
// expects.
//
// The API types the id as a string. In practice it is a decimal number, so it is parsed
// directly whenever possible: that keeps the real exchange id in the field, which is what shows
// up in logs and in the database, and it preserves ordering. Non-numeric ids are hashed with
// util.FNV64 instead, the way coinbase and kucoin handle their UUIDs, so that the conversion
// can never fail.
//
// The original string is always preserved in types.Order.UUID, and lookups should go through
// it, since the hashed form cannot be reversed.
func toGlobalOrderID(id string) uint64 {
	if orderID, err := strconv.ParseUint(id, 10, 64); err == nil {
		return orderID
	}

	return util.FNV64(id)
}

// toGlobalOrder converts a live order (submit / query / cancel responses).
func toGlobalOrder(o *backpackapi.Order) (*types.Order, error) {
	status, err := toGlobalOrderStatus(o.Status)
	if err != nil {
		return nil, err
	}

	return &types.Order{
		SubmitOrder: types.SubmitOrder{
			ClientOrderID: clientOrderIDString(o.ClientId),
			Symbol:        toGlobalSymbol(o.Symbol),
			Side:          toGlobalSide(o.Side),
			Type:          toGlobalOrderType(o.OrderType, o.PostOnly),
			Price:         o.Price,
			Quantity:      o.Quantity,
			TimeInForce:   toGlobalTimeInForce(o.TimeInForce),
			ReduceOnly:    o.ReduceOnly,
		},
		Exchange:         types.ExchangeBackpack,
		OrderID:          toGlobalOrderID(o.Id),
		UUID:             o.Id,
		Status:           status,
		OriginalStatus:   string(o.Status),
		ExecutedQuantity: o.ExecutedQuantity,
		IsWorking:        o.Status.IsWorking(),
		CreationTime:     types.Time(o.CreatedAt.Time()),
		UpdateTime:       types.Time(o.CreatedAt.Time()),
	}, nil
}

// orderHistoryEntryToGlobalOrder converts a historical order.
//
// It is separate from toGlobalOrder because the history endpoint encodes createdAt as a naive
// datetime string rather than a unix millisecond timestamp, and carries no update time.
func orderHistoryEntryToGlobalOrder(o *backpackapi.OrderHistoryEntry) (*types.Order, error) {
	status, err := toGlobalOrderStatus(o.Status)
	if err != nil {
		return nil, err
	}

	return &types.Order{
		SubmitOrder: types.SubmitOrder{
			ClientOrderID: clientOrderIDString(o.ClientId),
			Symbol:        toGlobalSymbol(o.Symbol),
			Side:          toGlobalSide(o.Side),
			Type:          toGlobalOrderType(o.OrderType, o.PostOnly),
			Price:         o.Price,
			Quantity:      o.Quantity,
			TimeInForce:   toGlobalTimeInForce(o.TimeInForce),
		},
		Exchange:         types.ExchangeBackpack,
		OrderID:          toGlobalOrderID(o.Id),
		UUID:             o.Id,
		Status:           status,
		OriginalStatus:   string(o.Status),
		ExecutedQuantity: o.ExecutedQuantity,
		IsWorking:        o.Status.IsWorking(),
		CreationTime:     types.Time(o.CreatedAt.Time()),
		UpdateTime:       types.Time(o.CreatedAt.Time()),
	}, nil
}

func toGlobalTrade(fill *backpackapi.OrderFill) types.Trade {
	side := toGlobalSide(fill.Side)

	return types.Trade{
		// TradeId is already an int64, only the order id needs converting
		ID:            uint64(fill.TradeId),
		OrderID:       toGlobalOrderID(fill.OrderId),
		OrderUUID:     fill.OrderId,
		Exchange:      types.ExchangeBackpack,
		Price:         fill.Price,
		Quantity:      fill.Quantity,
		QuoteQuantity: fill.Price.Mul(fill.Quantity),
		Symbol:        toGlobalSymbol(fill.Symbol),
		Side:          side,
		IsBuyer:       side == types.SideTypeBuy,
		// Backpack reports the maker flag directly, no taker/maker side inference needed
		IsMaker:     fill.IsMaker,
		Time:        types.Time(fill.Timestamp.Time()),
		Fee:         fill.Fee,
		FeeCurrency: fill.FeeSymbol,
	}
}

func toGlobalBalance(balances backpackapi.BalanceMap) types.BalanceMap {
	balanceMap := make(types.BalanceMap, len(balances))
	for currency, b := range balances {
		balance := types.NewZeroBalance(currency)
		balance.Available = b.Available
		// staked funds are not available for trading either
		balance.Locked = b.Locked.Add(b.Staked)
		balance.NetAsset = b.Total()
		balanceMap[currency] = balance
	}

	return balanceMap
}

func toGlobalTicker(ticker *backpackapi.Ticker, t time.Time) types.Ticker {
	return types.Ticker{
		Time:   t,
		Volume: ticker.Volume,
		Last:   ticker.LastPrice,
		Open:   ticker.FirstPrice,
		High:   ticker.High,
		Low:    ticker.Low,
		// Buy and Sell are left zero here: the Backpack ticker endpoint carries no bid/ask.
		// QueryTicker fills them from the order book; QueryTickers cannot, see its doc comment.
	}
}

func toGlobalKLine(symbol string, interval types.Interval, k *backpackapi.Kline) types.KLine {
	startTime := k.Start.Time()

	return types.KLine{
		Exchange:  types.ExchangeBackpack,
		Symbol:    symbol,
		StartTime: types.Time(startTime),
		// types.KLine.EndTime follows the binance rule: subtract a millisecond so that it does
		// not overlap the next interval's start time.
		EndTime:        types.Time(startTime.Add(interval.Duration() - time.Millisecond)),
		Interval:       interval,
		Open:           k.Open,
		Close:          k.Close,
		High:           k.High,
		Low:            k.Low,
		Volume:         k.Volume,
		QuoteVolume:    k.QuoteVolume,
		NumberOfTrades: uint64(k.Trades),
		Closed:         true,
	}
}

func toGlobalMarket(m *backpackapi.Market) types.Market {
	tickSize := m.Filters.Price.TickSize
	stepSize := m.Filters.Quantity.StepSize

	return types.Market{
		Exchange:    types.ExchangeBackpack,
		Symbol:      toGlobalSymbol(m.Symbol),
		LocalSymbol: m.Symbol,

		BaseCurrency:  m.BaseSymbol,
		QuoteCurrency: m.QuoteSymbol,

		PricePrecision:  tickSize.NumFractionalDigits(),
		VolumePrecision: stepSize.NumFractionalDigits(),

		TickSize: tickSize,
		StepSize: stepSize,

		MinQuantity: m.Filters.Quantity.MinQuantity,
		MaxQuantity: m.Filters.Quantity.MaxQuantity,
		MinPrice:    m.Filters.Price.MinPrice,
		MaxPrice:    m.Filters.Price.MaxPrice,

		// Backpack does not publish a minimal notional, use 1 USD like the other adapters.
		MinNotional: fixedpoint.One,
		MinAmount:   fixedpoint.One,
	}
}

// clientOrderIDString renders the numeric Backpack client order id as a string, mapping the
// unset value (0) to an empty string so that it round-trips through types.SubmitOrder.
func clientOrderIDString(clientID uint32) string {
	if clientID == 0 {
		return ""
	}

	return fmt.Sprintf("%d", clientID)
}
