package okex

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/c9s/bbgo/pkg/exchange/okex/okexapi"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func toGlobalSymbol(symbol string) string {
	symbol = strings.TrimSuffix(symbol, "-SWAP")
	return strings.ReplaceAll(symbol, "-", "")
}

//go:generate go run gensymbols.go
func toLocalSymbol(symbol string, instType ...okexapi.InstrumentType) string {
	if len(instType) == 1 {
		if instType[0] == okexapi.InstrumentTypeSwap {
			if s, ok := swapSymbolMap[symbol]; ok {
				return s
			}
		}
	}

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
	for _, detail := range account.Details {

		balanceMap[detail.Currency] = types.Balance{
			Currency:  detail.Currency,
			Available: detail.Available,
			Locked:    detail.FrozenBalance,
			Interest:  detail.Interest,        // accrued interest
			Borrowed:  detail.Liability.Abs(), // okx liability does not include the accrued interest
			NetAsset:  detail.Equity,
		}
	}
	return balanceMap
}

type WebsocketSubscription struct {
	Channel        Channel `json:"channel"`
	InstrumentID   string  `json:"instId,omitempty"`
	InstrumentType string  `json:"instType,omitempty"`
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
	switch s.Channel {
	case types.KLineChannel:
		// Channel names are:
		return WebsocketSubscription{
			Channel:      Channel(convertIntervalToCandle(s.Options.Interval)),
			InstrumentID: toLocalSymbol(s.Symbol),
		}, nil

	case types.BookChannel:
		ch := ChannelBooks

		switch s.Options.Depth {
		case types.DepthLevelFull:
			ch = ChannelBooks

		case types.DepthLevelMedium:
			ch = ChannelBooks50

		case types.DepthLevel50:
			ch = ChannelBooks50

		case types.DepthLevel5:
			ch = ChannelBooks5

		case types.DepthLevel1:
			ch = ChannelBooks1
		}

		return WebsocketSubscription{
			Channel:      ch,
			InstrumentID: toLocalSymbol(s.Symbol),
		}, nil
	case types.BookTickerChannel:
		return WebsocketSubscription{
			Channel:      ChannelBooks5,
			InstrumentID: toLocalSymbol(s.Symbol),
		}, nil
	case types.MarketTradeChannel:
		return WebsocketSubscription{
			Channel:      ChannelMarketTrades,
			InstrumentID: toLocalSymbol(s.Symbol),
		}, nil
	}

	return WebsocketSubscription{}, fmt.Errorf("unsupported public stream channel %s", s.Channel)
}

func toLocalSideType(side types.SideType) okexapi.SideType {
	return okexapi.SideType(strings.ToLower(string(side)))
}

func toGlobalTrade(trade okexapi.Trade) types.Trade {
	side := toGlobalSide(trade.Side)
	return types.Trade{
		ID:            uint64(trade.TradeId),
		OrderID:       uint64(trade.OrderId),
		Exchange:      types.ExchangeOKEx,
		Price:         trade.FillPrice,
		Quantity:      trade.FillSize,
		QuoteQuantity: trade.FillPrice.Mul(trade.FillSize),
		Symbol:        toGlobalSymbol(trade.InstrumentId),
		Side:          side,
		IsBuyer:       side == types.SideTypeBuy,
		IsMaker:       trade.ExecutionType == okexapi.LiquidityTypeMaker,
		Time:          types.Time(trade.Timestamp),
		// The fees obtained from the exchange are negative, hence they are forcibly converted to positive.
		Fee:         trade.Fee.Abs(),
		FeeCurrency: trade.FeeCurrency,
		IsMargin:    trade.InstrumentType == okexapi.InstrumentTypeMargin,
		IsFutures: trade.InstrumentType == okexapi.InstrumentTypeFutures ||
			trade.InstrumentType == okexapi.InstrumentTypeSwap,
		IsIsolated: false,
	}
}

func processMarketBuySize(o *okexapi.OrderDetail) (fixedpoint.Value, error) {
	switch o.State {
	case okexapi.OrderStateLive, okexapi.OrderStateCanceled:
		return fixedpoint.Zero, nil

	case okexapi.OrderStatePartiallyFilled:
		if o.FillPrice.IsZero() {
			return fixedpoint.Zero, fmt.Errorf("fillPrice for a partialFilled should not be zero")
		}
		return o.Size.Div(o.FillPrice), nil

	case okexapi.OrderStateFilled:
		return o.AccumulatedFillSize, nil

	default:
		return fixedpoint.Zero, fmt.Errorf("unexpected status: %s", o.State)
	}
}

func orderDetailToGlobalOrder(order *okexapi.OrderDetail) (*types.Order, error) {
	side := toGlobalSide(order.Side)

	orderType, err := toGlobalOrderType(order.OrderType)
	if err != nil {
		return nil, err
	}

	timeInForce := types.TimeInForceGTC
	switch order.OrderType {
	case okexapi.OrderTypeFOK:
		timeInForce = types.TimeInForceFOK
	case okexapi.OrderTypeIOC:
		timeInForce = types.TimeInForceIOC
	}

	orderStatus, err := toGlobalOrderStatus(order.State)
	if err != nil {
		return nil, err
	}

	size := order.Size
	if order.Side == okexapi.SideTypeBuy &&
		order.OrderType == okexapi.OrderTypeMarket &&
		order.TargetCurrency == okexapi.TargetCurrencyQuote {

		size, err = processMarketBuySize(order)
		if err != nil {
			return nil, err
		}
	}

	return &types.Order{
		SubmitOrder: types.SubmitOrder{
			ClientOrderID: order.ClientOrderId,
			Symbol:        toGlobalSymbol(order.InstrumentID),
			Side:          side,
			Type:          orderType,
			Price:         order.Price,
			Quantity:      size,
			AveragePrice:  order.AvgPrice,
			TimeInForce:   timeInForce,
		},
		Exchange:         types.ExchangeOKEx,
		OrderID:          uint64(order.OrderId),
		UUID:             strconv.FormatInt(int64(order.OrderId), 10),
		Status:           orderStatus,
		OriginalStatus:   string(order.State),
		ExecutedQuantity: order.AccumulatedFillSize,
		IsWorking:        order.State.IsWorking(),
		CreationTime:     types.Time(order.CreatedTime),
		UpdateTime:       types.Time(order.UpdatedTime),
		IsMargin:         order.InstrumentType == okexapi.InstrumentTypeMargin,
		IsFutures: order.InstrumentType == okexapi.InstrumentTypeFutures ||
			order.InstrumentType == okexapi.InstrumentTypeSwap,
	}, nil
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

var localOrderTypeMap = map[types.OrderType]okexapi.OrderType{
	types.OrderTypeMarket:     okexapi.OrderTypeMarket,
	types.OrderTypeLimit:      okexapi.OrderTypeLimit,
	types.OrderTypeLimitMaker: okexapi.OrderTypePostOnly,
}

func toLocalOrderType(orderType types.OrderType) (okexapi.OrderType, error) {
	if ot, ok := localOrderTypeMap[orderType]; ok {
		return ot, nil
	}

	return "", fmt.Errorf("unknown or unsupported okex order type: %s", orderType)
}

var globalOrderTypeMap = map[okexapi.OrderType]types.OrderType{
	okexapi.OrderTypeMarket:   types.OrderTypeMarket,
	okexapi.OrderTypeLimit:    types.OrderTypeLimit,
	okexapi.OrderTypePostOnly: types.OrderTypeLimitMaker,
	okexapi.OrderTypeFOK:      types.OrderTypeLimit,
	okexapi.OrderTypeIOC:      types.OrderTypeLimit,
}

func toGlobalOrderType(orderType okexapi.OrderType) (types.OrderType, error) {
	// IOC, FOK are only allowed with limit order type, so we assume the order type is always limit order for FOK, IOC orders
	if ot, ok := globalOrderTypeMap[orderType]; ok {
		return ot, nil
	}

	return "", fmt.Errorf("unknown or unsupported okex order type: %s", orderType)
}

func toLocalInterval(interval types.Interval) (string, error) {
	if _, ok := SupportedIntervals[interval]; !ok {
		return "", fmt.Errorf("interval %s is not supported", interval)
	}

	in, ok := ToLocalInterval[interval]
	if !ok {
		return "", fmt.Errorf("interval %s is not supported, got local interval %s", interval, in)
	}

	return in, nil
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
	if okexOrder.InstrumentType == okexapi.InstrumentTypeMargin {
		isMargin = true
	}

	isFutures := false
	if okexOrder.InstrumentType == okexapi.InstrumentTypeFutures ||
		okexOrder.InstrumentType == okexapi.InstrumentTypeSwap {
		isFutures = true
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
		IsFutures:        isFutures,
	}, nil
}

func toGlobalMarginLoan(record okexapi.MarginHistoryEntry) types.MarginLoan {
	return types.MarginLoan{
		Exchange:       types.ExchangeOKEx,
		TransactionID:  uint64(record.Ts.Time().UnixMilli()),
		Asset:          record.Currency,
		Principle:      record.Amount,
		Time:           types.Time(record.Ts.Time()),
		IsolatedSymbol: "",
	}
}

func toGlobalMarginRepay(record okexapi.MarginHistoryEntry) types.MarginRepay {
	return types.MarginRepay{
		Exchange:       types.ExchangeOKEx,
		TransactionID:  uint64(record.Ts.Time().UnixMilli()),
		Asset:          record.Currency,
		Principle:      record.Amount,
		Time:           types.Time(record.Ts.Time()),
		IsolatedSymbol: "",
	}
}

// DepositRecord.state represents the deposit state
/*
Status of deposit
0: Waiting for confirmation
1: Deposit credited
2: Deposit successful
8: Pending due to temporary deposit suspension on this crypto currency
11: Match the address blacklist
12: Account or deposit is frozen
13: Sub-account deposit interception
14: KYC limit
*/
func toDepositStatusMessage(state int64) string {
	switch state {
	case 0:
		return "Waiting for confirmation"
	case 1:
		return "Deposit credited"
	case 2:
		return "Deposit successful"
	case 8:
		return "Pending due to temporary deposit suspension on this crypto currency"
	case 11:
		return "Match the address blacklist"
	case 12:
		return "Account or deposit is frozen"
	case 13:
		return "Sub-account deposit interception"
	case 14:
		return "KYC limit"
	}

	return ""
}

func toGlobalDepositStatus(state int64) types.DepositStatus {
	switch state {
	case 0:
		return types.DepositPending
	case 1:
		return types.DepositCredited
	case 2:
		return types.DepositSuccess
	case 8:
		return types.DepositPending
	default:
		return types.DepositRejected
	}
}

func toGlobalDeposit(record okexapi.DepositRecord) types.Deposit {
	return types.Deposit{
		Exchange:      types.ExchangeOKEx,
		Time:          types.Time(record.Ts),
		Amount:        record.Amount,
		Asset:         record.Currency,
		Address:       record.To,
		AddressTag:    "",
		TransactionID: record.DepId,
		Status:        toGlobalDepositStatus(int64(record.State)),
		RawStatus:     fmt.Sprintf("%s (%s)", record.State.String(), toDepositStatusMessage(int64(record.State))),
		UnlockConfirm: 0,
		Confirmation:  record.ActualDepBlkConfirm.String(),
		Network:       strings.ToUpper(record.Chain),
	}

}
