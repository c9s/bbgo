package okex

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"go.uber.org/multierr"

	"github.com/c9s/bbgo/pkg/exchange/okex/okexapi"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func toGlobalSymbol(symbol string) string {
	return strings.ReplaceAll(symbol, "-", "")
}

// //go:generate sh -c "echo \"package okex\nvar spotSymbolMap = map[string]string{\n\" $(curl -s -L 'https://okex.com/api/v5/public/instruments?instType=SPOT' | jq -r '.data[] | \"\\(.instId | sub(\"-\" ; \"\") | tojson ): \\( .instId | tojson),\n\"') \"\n}\" > symbols.go"
//
//go:generate go run gensymbols.go
func toLocalSymbol(symbol string) string {
	if s, ok := spotSymbolMap[symbol]; ok {
		return s
	}

	log.Errorf("failed to look up local symbol from %s", symbol)
	return symbol
}

func toGlobalTicker(marketTicker okexapi.MarketTicker) *types.Ticker {
	return &types.Ticker{
		Time:   marketTicker.Timestamp.Time(),
		Volume: marketTicker.Volume24H,
		Last:   marketTicker.Last,
		Open:   marketTicker.Open24H,
		High:   marketTicker.High24H,
		Low:    marketTicker.Low24H,
		Buy:    marketTicker.BidPrice,
		Sell:   marketTicker.AskPrice,
	}
}

func toGlobalBalance(account *okexapi.Account) types.BalanceMap {
	var balanceMap = types.BalanceMap{}
	for _, balanceDetail := range account.Details {
		balanceMap[balanceDetail.Currency] = types.Balance{
			Currency:  balanceDetail.Currency,
			Available: balanceDetail.CashBalance,
			Locked:    balanceDetail.Frozen,
		}
	}
	return balanceMap
}

type WebsocketSubscription struct {
	Channel        string `json:"channel"`
	InstrumentID   string `json:"instId,omitempty"`
	InstrumentType string `json:"instType,omitempty"`
}

var CandleChannels = []string{
	"candle1Y",
	"candle6M", "candle3M", "candle1M",
	"candle1W",
	"candle1D", "candle2D", "candle3D", "candle5D",
	"candle12H", "candle6H", "candle4H", "candle2H", "candle1H",
	"candle30m", "candle15m", "candle5m", "candle3m", "candle1m",
}

func convertIntervalToCandle(interval types.Interval) string {
	s := interval.String()
	switch s {

	case "1h", "2h", "4h", "6h", "12h", "1d", "3d":
		return "candle" + strings.ToUpper(s)

	case "1m", "5m", "15m", "30m":
		return "candle" + s

	}

	return "candle" + s
}

func convertSubscription(s types.Subscription) (WebsocketSubscription, error) {
	// binance uses lower case symbol name,
	// for kline, it's "<symbol>@kline_<interval>"
	// for depth, it's "<symbol>@depth OR <symbol>@depth@100ms"
	switch s.Channel {
	case types.KLineChannel:
		// Channel names are:
		return WebsocketSubscription{
			Channel:      convertIntervalToCandle(s.Options.Interval),
			InstrumentID: toLocalSymbol(s.Symbol),
		}, nil

	case types.BookChannel:
		return WebsocketSubscription{
			Channel:      "books",
			InstrumentID: toLocalSymbol(s.Symbol),
		}, nil
	case types.BookTickerChannel:
		return WebsocketSubscription{
			Channel:      "books5",
			InstrumentID: toLocalSymbol(s.Symbol),
		}, nil
	}

	return WebsocketSubscription{}, fmt.Errorf("unsupported public stream channel %s", s.Channel)
}

func toLocalSideType(side types.SideType) okexapi.SideType {
	return okexapi.SideType(strings.ToLower(string(side)))
}

func segmentOrderDetails(orderDetails []okexapi.OrderDetails) (trades, orders []okexapi.OrderDetails) {
	for _, orderDetail := range orderDetails {
		if len(orderDetail.LastTradeID) > 0 {
			trades = append(trades, orderDetail)
		}
		orders = append(orders, orderDetail)
	}
	return trades, orders
}

func toGlobalTrades(orderDetails []okexapi.OrderDetails) ([]types.Trade, error) {
	var trades []types.Trade
	for _, orderDetail := range orderDetails {
		tradeID, err := strconv.ParseInt(orderDetail.LastTradeID, 10, 64)
		if err != nil {
			return trades, errors.Wrapf(err, "error parsing tradeId value: %s", orderDetail.LastTradeID)
		}

		orderID, err := strconv.ParseInt(orderDetail.OrderID, 10, 64)
		if err != nil {
			return trades, errors.Wrapf(err, "error parsing ordId value: %s", orderDetail.OrderID)
		}

		side := types.SideType(strings.ToUpper(string(orderDetail.Side)))

		trades = append(trades, types.Trade{
			ID:            uint64(tradeID),
			OrderID:       uint64(orderID),
			Exchange:      types.ExchangeOKEx,
			Price:         orderDetail.LastFilledPrice,
			Quantity:      orderDetail.LastFilledQuantity,
			QuoteQuantity: orderDetail.LastFilledPrice.Mul(orderDetail.LastFilledQuantity),
			Symbol:        toGlobalSymbol(orderDetail.InstrumentID),
			Side:          side,
			IsBuyer:       side == types.SideTypeBuy,
			IsMaker:       orderDetail.ExecutionType == "M",
			Time:          types.Time(orderDetail.LastFilledTime),
			Fee:           orderDetail.LastFilledFee,
			FeeCurrency:   orderDetail.LastFilledFeeCurrency,
			IsMargin:      false,
			IsIsolated:    false,
		})
	}

	return trades, nil
}

func toGlobalOrders(orderDetails []okexapi.OrderDetails) ([]types.Order, error) {
	var orders []types.Order
	var err error
	for _, orderDetail := range orderDetails {

		o, err2 := toGlobalOrder(&orderDetail)
		if err2 != nil {
			err = multierr.Append(err, err2)
			continue
		}
		orders = append(orders, *o)
	}

	return orders, err
}

func toGlobalOrderStatus(state okexapi.OrderState) (types.OrderStatus, error) {
	switch state {
	case okexapi.OrderStateCanceled:
		return types.OrderStatusCanceled, nil
	case okexapi.OrderStateLive:
		return types.OrderStatusNew, nil
	case okexapi.OrderStatePartiallyFilled:
		return types.OrderStatusPartiallyFilled, nil
	case okexapi.OrderStateFilled:
		return types.OrderStatusFilled, nil

	}

	return "", fmt.Errorf("unknown or unsupported okex order state: %s", state)
}

func toLocalOrderType(orderType types.OrderType) (okexapi.OrderType, error) {
	switch orderType {
	case types.OrderTypeMarket:
		return okexapi.OrderTypeMarket, nil

	case types.OrderTypeLimit:
		return okexapi.OrderTypeLimit, nil

	case types.OrderTypeLimitMaker:
		return okexapi.OrderTypePostOnly, nil

	}

	return "", fmt.Errorf("unknown or unsupported okex order type: %s", orderType)
}

func toGlobalOrderType(orderType okexapi.OrderType) (types.OrderType, error) {
	// IOC, FOK are only allowed with limit order type, so we assume the order type is always limit order for FOK, IOC orders
	switch orderType {
	case okexapi.OrderTypeMarket:
		return types.OrderTypeMarket, nil

	case okexapi.OrderTypeLimit, okexapi.OrderTypeFOK, okexapi.OrderTypeIOC:
		return types.OrderTypeLimit, nil

	case okexapi.OrderTypePostOnly:
		return types.OrderTypeLimitMaker, nil

	}

	return "", fmt.Errorf("unknown or unsupported okex order type: %s", orderType)
}

func toLocalInterval(src string) string {
	var re = regexp.MustCompile(`\d+[hdw]`)
	return re.ReplaceAllStringFunc(src, func(w string) string {
		return strings.ToUpper(w)
	})
}

func toGlobalSide(side okexapi.SideType) (s types.SideType) {
	switch string(side) {
	case "sell":
		s = types.SideTypeSell
	case "buy":
		s = types.SideTypeBuy
	}
	return s
}

func toGlobalOrder(okexOrder *okexapi.OrderDetails) (*types.Order, error) {

	orderID, err := strconv.ParseInt(okexOrder.OrderID, 10, 64)
	if err != nil {
		return nil, err
	}

	side := toGlobalSide(okexOrder.Side)

	orderType, err := toGlobalOrderType(okexOrder.OrderType)
	if err != nil {
		return nil, err
	}

	timeInForce := types.TimeInForceGTC
	switch okexOrder.OrderType {
	case okexapi.OrderTypeFOK:
		timeInForce = types.TimeInForceFOK
	case okexapi.OrderTypeIOC:
		timeInForce = types.TimeInForceIOC
	}

	orderStatus, err := toGlobalOrderStatus(okexOrder.State)
	if err != nil {
		return nil, err
	}

	isWorking := false
	switch orderStatus {
	case types.OrderStatusNew, types.OrderStatusPartiallyFilled:
		isWorking = true

	}

	isMargin := false
	if okexOrder.InstrumentType == string(okexapi.InstrumentTypeMARGIN) {
		isMargin = true
	}

	return &types.Order{
		SubmitOrder: types.SubmitOrder{
			ClientOrderID: okexOrder.ClientOrderID,
			Symbol:        toGlobalSymbol(okexOrder.InstrumentID),
			Side:          side,
			Type:          orderType,
			Price:         okexOrder.Price,
			Quantity:      okexOrder.Quantity,
			StopPrice:     fixedpoint.Zero, // not supported yet
			TimeInForce:   timeInForce,
		},
		Exchange:         types.ExchangeOKEx,
		OrderID:          uint64(orderID),
		Status:           orderStatus,
		ExecutedQuantity: okexOrder.FilledQuantity,
		IsWorking:        isWorking,
		CreationTime:     types.Time(okexOrder.CreationTime),
		UpdateTime:       types.Time(okexOrder.UpdateTime),
		IsMargin:         isMargin,
		IsIsolated:       false,
	}, nil
}
