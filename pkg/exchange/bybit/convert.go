package bybit

import (
	"fmt"
	"strconv"
	"time"

	"github.com/c9s/bbgo/pkg/exchange/bybit/bybitapi"
	"github.com/c9s/bbgo/pkg/exchange/bybit/bybitapi/v3"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func toGlobalMarket(m bybitapi.Instrument) types.Market {
	return types.Market{
		Symbol:          m.Symbol,
		LocalSymbol:     m.Symbol,
		PricePrecision:  m.LotSizeFilter.QuotePrecision.NumFractionalDigits(),
		VolumePrecision: m.LotSizeFilter.BasePrecision.NumFractionalDigits(),
		QuoteCurrency:   m.QuoteCoin,
		BaseCurrency:    m.BaseCoin,
		MinNotional:     m.LotSizeFilter.MinOrderAmt,
		MinAmount:       m.LotSizeFilter.MinOrderAmt,

		// quantity
		MinQuantity: m.LotSizeFilter.MinOrderQty,
		MaxQuantity: m.LotSizeFilter.MaxOrderQty,
		StepSize:    m.LotSizeFilter.BasePrecision,

		// price
		MinPrice: m.LotSizeFilter.MinOrderAmt,
		MaxPrice: m.LotSizeFilter.MaxOrderAmt,
		TickSize: m.PriceFilter.TickSize,
	}
}

func toGlobalTicker(stats bybitapi.Ticker, time time.Time) types.Ticker {
	return types.Ticker{
		Volume: stats.Volume24H,
		Last:   stats.LastPrice,
		Open:   stats.PrevPrice24H, // Market price 24 hours ago
		High:   stats.HighPrice24H,
		Low:    stats.LowPrice24H,
		Buy:    stats.Bid1Price,
		Sell:   stats.Ask1Price,
		Time:   time,
	}
}

func toGlobalOrder(order bybitapi.Order) (*types.Order, error) {
	side, err := toGlobalSideType(order.Side)
	if err != nil {
		return nil, err
	}

	orderType, err := toGlobalOrderType(order.OrderType)
	if err != nil {
		return nil, err
	}

	timeInForce, err := toGlobalTimeInForce(order.TimeInForce)
	if err != nil {
		return nil, err
	}

	status, err := toGlobalOrderStatus(order.OrderStatus, order.Side, order.OrderType)
	if err != nil {
		return nil, err
	}

	// linear and inverse : 42f4f364-82e1-49d3-ad1d-cd8cf9aa308d (UUID format)
	// spot : 1468264727470772736 (only numbers)
	// Now we only use spot trading.
	orderIdNum, err := strconv.ParseUint(order.OrderId, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("unexpected order id: %s, err: %w", order.OrderId, err)
	}

	qty, err := processMarketBuyQuantity(order)
	if err != nil {
		return nil, err
	}

	return &types.Order{
		SubmitOrder: types.SubmitOrder{
			ClientOrderID: order.OrderLinkId,
			Symbol:        order.Symbol,
			Side:          side,
			Type:          orderType,
			Quantity:      qty,
			Price:         order.Price,
			TimeInForce:   timeInForce,
		},
		Exchange:         types.ExchangeBybit,
		OrderID:          orderIdNum,
		UUID:             order.OrderId,
		Status:           status,
		ExecutedQuantity: order.CumExecQty,
		IsWorking:        status == types.OrderStatusNew || status == types.OrderStatusPartiallyFilled,
		CreationTime:     types.Time(order.CreatedTime.Time()),
		UpdateTime:       types.Time(order.UpdatedTime.Time()),
	}, nil
}

func toGlobalSideType(side bybitapi.Side) (types.SideType, error) {
	switch side {
	case bybitapi.SideBuy:
		return types.SideTypeBuy, nil

	case bybitapi.SideSell:
		return types.SideTypeSell, nil

	default:
		return types.SideType(side), fmt.Errorf("unexpected side: %s", side)
	}
}

func toGlobalOrderType(s bybitapi.OrderType) (types.OrderType, error) {
	switch s {
	case bybitapi.OrderTypeMarket:
		return types.OrderTypeMarket, nil

	case bybitapi.OrderTypeLimit:
		return types.OrderTypeLimit, nil

	default:
		return types.OrderType(s), fmt.Errorf("unexpected order type: %s", s)
	}
}

func toGlobalTimeInForce(force bybitapi.TimeInForce) (types.TimeInForce, error) {
	switch force {
	case bybitapi.TimeInForceGTC:
		return types.TimeInForceGTC, nil

	case bybitapi.TimeInForceIOC:
		return types.TimeInForceIOC, nil

	case bybitapi.TimeInForceFOK:
		return types.TimeInForceFOK, nil

	default:
		return types.TimeInForce(force), fmt.Errorf("unexpected timeInForce type: %s", force)
	}
}

func toGlobalOrderStatus(status bybitapi.OrderStatus, side bybitapi.Side, orderType bybitapi.OrderType) (types.OrderStatus, error) {
	switch status {

	case bybitapi.OrderStatusPartiallyFilledCanceled:
		// market buy order	-> PartiallyFilled -> PartiallyFilledCanceled
		if orderType == bybitapi.OrderTypeMarket && side == bybitapi.SideBuy {
			return types.OrderStatusFilled, nil
		}
		// limit buy/sell order -> PartiallyFilled -> PartiallyFilledCanceled(Canceled)
		return types.OrderStatusCanceled, nil

	default:
		return processOtherOrderStatus(status)
	}
}

func processOtherOrderStatus(status bybitapi.OrderStatus) (types.OrderStatus, error) {
	switch status {
	case bybitapi.OrderStatusCreated,
		bybitapi.OrderStatusNew,
		bybitapi.OrderStatusActive:
		return types.OrderStatusNew, nil

	case bybitapi.OrderStatusFilled:
		return types.OrderStatusFilled, nil

	case bybitapi.OrderStatusPartiallyFilled:
		return types.OrderStatusPartiallyFilled, nil

	case bybitapi.OrderStatusCancelled,
		bybitapi.OrderStatusDeactivated:
		return types.OrderStatusCanceled, nil

	case bybitapi.OrderStatusRejected:
		return types.OrderStatusRejected, nil

	default:
		// following not supported
		// bybitapi.OrderStatusUntriggered
		// bybitapi.OrderStatusTriggered
		return types.OrderStatus(status), fmt.Errorf("unexpected order status: %s", status)
	}
}

// processMarketBuyQuantity converts the quantity unit from quote coin to base coin if the order is a **MARKET BUY**.
//
// If the status is OrderStatusPartiallyFilled, it returns the estimated quantity based on the base coin.
//
// If the order status is OrderStatusPartiallyFilledCanceled, it indicates that the order is not fully filled,
// and the system has automatically canceled it. In this scenario, CumExecQty is considered equal to Qty.
func processMarketBuyQuantity(o bybitapi.Order) (fixedpoint.Value, error) {
	if o.Side != bybitapi.SideBuy || o.OrderType != bybitapi.OrderTypeMarket {
		return o.Qty, nil
	}

	var qty fixedpoint.Value
	switch o.OrderStatus {
	case bybitapi.OrderStatusPartiallyFilled:
		// if CumExecValue is zero, it indicates the caller is from the RESTFUL API.
		// we can use AvgPrice to estimate quantity.
		if o.CumExecValue.IsZero() {
			if o.AvgPrice.IsZero() {
				return fixedpoint.Zero, fmt.Errorf("AvgPrice shouldn't be zero")
			}

			qty = o.Qty.Div(o.AvgPrice)
		} else {
			if o.CumExecQty.IsZero() {
				return fixedpoint.Zero, fmt.Errorf("CumExecQty shouldn't be zero")
			}

			// from web socket event
			qty = o.Qty.Div(o.CumExecValue.Div(o.CumExecQty))
		}

	case bybitapi.OrderStatusPartiallyFilledCanceled,
		// Considering extreme scenarios, there's a possibility that 'OrderStatusFilled' could occur.
		bybitapi.OrderStatusFilled:
		qty = o.CumExecQty

	case bybitapi.OrderStatusCreated,
		bybitapi.OrderStatusNew,
		bybitapi.OrderStatusRejected:
		qty = fixedpoint.Zero

	case bybitapi.OrderStatusCancelled:
		qty = o.Qty

	default:
		return fixedpoint.Zero, fmt.Errorf("unexpected order status: %s", o.OrderStatus)
	}

	return qty, nil
}

func toLocalOrderType(orderType types.OrderType) (bybitapi.OrderType, error) {
	switch orderType {
	case types.OrderTypeLimit:
		return bybitapi.OrderTypeLimit, nil

	case types.OrderTypeMarket:
		return bybitapi.OrderTypeMarket, nil

	default:
		return "", fmt.Errorf("order type %s not supported", orderType)
	}
}

func toLocalSide(side types.SideType) (bybitapi.Side, error) {
	switch side {
	case types.SideTypeSell:
		return bybitapi.SideSell, nil

	case types.SideTypeBuy:
		return bybitapi.SideBuy, nil

	default:
		return "", fmt.Errorf("side type %s not supported", side)
	}
}

func toV3Buyer(isBuyer v3.Side) (types.SideType, error) {
	switch isBuyer {
	case v3.SideBuy:
		return types.SideTypeBuy, nil
	case v3.SideSell:
		return types.SideTypeSell, nil
	default:
		return "", fmt.Errorf("unexpected side type: %s", isBuyer)
	}
}
func toV3Maker(isMaker v3.OrderType) (bool, error) {
	switch isMaker {
	case v3.OrderTypeMaker:
		return true, nil
	case v3.OrderTypeTaker:
		return false, nil
	default:
		return false, fmt.Errorf("unexpected order type: %s", isMaker)
	}
}

func v3ToGlobalTrade(trade v3.Trade) (*types.Trade, error) {
	side, err := toV3Buyer(trade.IsBuyer)
	if err != nil {
		return nil, err
	}
	isMaker, err := toV3Maker(trade.IsMaker)
	if err != nil {
		return nil, err
	}

	orderIdNum, err := strconv.ParseUint(trade.OrderId, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("unexpected order id: %s, err: %w", trade.OrderId, err)
	}
	tradeIdNum, err := strconv.ParseUint(trade.TradeId, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("unexpected trade id: %s, err: %w", trade.TradeId, err)
	}

	return &types.Trade{
		ID:            tradeIdNum,
		OrderID:       orderIdNum,
		Exchange:      types.ExchangeBybit,
		Price:         trade.OrderPrice,
		Quantity:      trade.OrderQty,
		QuoteQuantity: trade.OrderPrice.Mul(trade.OrderQty),
		Symbol:        trade.Symbol,
		Side:          side,
		IsBuyer:       side == types.SideTypeBuy,
		IsMaker:       isMaker,
		Time:          types.Time(trade.ExecutionTime),
		Fee:           trade.ExecFee,
		FeeCurrency:   trade.FeeTokenId,
		IsMargin:      false,
		IsFutures:     false,
		IsIsolated:    false,
	}, nil
}

func toGlobalBalanceMap(events []bybitapi.WalletBalances) types.BalanceMap {
	bm := types.BalanceMap{}
	for _, event := range events {
		if event.AccountType != bybitapi.AccountTypeSpot {
			continue
		}

		for _, obj := range event.Coins {
			bm[obj.Coin] = types.Balance{
				Currency:  obj.Coin,
				Available: obj.Free,
				Locked:    obj.Locked,
			}
		}
	}
	return bm
}

func toLocalInterval(interval types.Interval) (string, error) {
	if _, found := bybitapi.SupportedIntervals[interval]; !found {
		return "", fmt.Errorf("interval not supported: %s", interval)
	}

	switch interval {

	case types.Interval1d:
		return string(bybitapi.IntervalSignDay), nil

	case types.Interval1w:
		return string(bybitapi.IntervalSignWeek), nil

	case types.Interval1mo:
		return string(bybitapi.IntervalSignMonth), nil

	default:
		return fmt.Sprintf("%d", interval.Minutes()), nil

	}
}

func toGlobalKLines(symbol string, interval types.Interval, klines []bybitapi.KLine) []types.KLine {
	gKLines := make([]types.KLine, len(klines))
	for i, kline := range klines {
		endTime := types.Time(kline.StartTime.Time().Add(interval.Duration() - time.Millisecond))
		gKLines[i] = types.KLine{
			Exchange:    types.ExchangeBybit,
			Symbol:      symbol,
			StartTime:   types.Time(kline.StartTime),
			EndTime:     endTime,
			Interval:    interval,
			Open:        kline.Open,
			Close:       kline.Close,
			High:        kline.High,
			Low:         kline.Low,
			Volume:      kline.Volume,
			QuoteVolume: kline.TurnOver,
			// Bybit doesn't support close flag in REST API
			Closed: false,
		}
	}
	return gKLines
}
