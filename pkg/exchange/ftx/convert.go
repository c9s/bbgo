package ftx

import (
	"fmt"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func toGlobalCurrency(original string) string {
	return TrimUpperString(original)
}

func toGlobalSymbol(original string) string {
	return strings.ReplaceAll(TrimUpperString(original), "/", "")
}

func toLocalSymbol(original string) string {
	if symbolMap[original] == "" {
		return original
	}

	return symbolMap[original]
}

func TrimUpperString(original string) string {
	return strings.ToUpper(strings.TrimSpace(original))
}

func TrimLowerString(original string) string {
	return strings.ToLower(strings.TrimSpace(original))
}

var errUnsupportedOrderStatus = fmt.Errorf("unsupported order status")

func toGlobalOrder(r order) (types.Order, error) {
	// In exchange/max/convert.go, it only parses these fields.
	o := types.Order{
		SubmitOrder: types.SubmitOrder{
			ClientOrderID: r.ClientId,
			Symbol:        toGlobalSymbol(r.Market),
			Side:          types.SideType(TrimUpperString(r.Side)),
			// order type definition: https://github.com/ftexchange/ftx/blob/master/rest/client.py#L122
			Type:        types.OrderType(TrimUpperString(r.Type)),
			Quantity:    r.Size,
			Price:       r.Price,
			TimeInForce: "GTC",
		},
		Exchange:         types.ExchangeFTX,
		IsWorking:        r.Status == "open",
		OrderID:          uint64(r.ID),
		Status:           "",
		ExecutedQuantity: r.FilledSize,
		CreationTime:     types.Time(r.CreatedAt.Time),
		UpdateTime:       types.Time(r.CreatedAt.Time),
	}

	// `new` (accepted but not processed yet), `open`, or `closed` (filled or cancelled)
	switch r.Status {
	case "new":
		o.Status = types.OrderStatusNew
	case "open":
		if fixedpoint.NewFromFloat(o.ExecutedQuantity) != fixedpoint.NewFromInt(0) {
			o.Status = types.OrderStatusPartiallyFilled
		} else {
			o.Status = types.OrderStatusNew
		}
	case "closed":
		// filled or canceled
		if fixedpoint.NewFromFloat(o.Quantity) == fixedpoint.NewFromFloat(o.ExecutedQuantity) {
			o.Status = types.OrderStatusFilled
		} else {
			// can't distinguish it's canceled or rejected from order response, so always set to canceled
			o.Status = types.OrderStatusCanceled
		}
	default:
		return types.Order{}, fmt.Errorf("unsupported status %s: %w", r.Status, errUnsupportedOrderStatus)
	}

	return o, nil
}

func toGlobalDeposit(input depositHistory) (types.Deposit, error) {
	s, err := toGlobalDepositStatus(input.Status)
	if err != nil {
		log.WithError(err).Warnf("assign empty string to the deposit status")
	}
	t := input.Time
	if input.ConfirmedTime.Time != (time.Time{}) {
		t = input.ConfirmedTime
	}
	d := types.Deposit{
		GID:           0,
		Exchange:      types.ExchangeFTX,
		Time:          types.Time(t.Time),
		Amount:        input.Size,
		Asset:         toGlobalCurrency(input.Coin),
		TransactionID: input.TxID,
		Status:        s,
		Address:       input.Address.Address,
		AddressTag:    input.Address.Tag,
	}
	return d, nil
}

func toGlobalDepositStatus(input string) (types.DepositStatus, error) {
	// The document only list `confirmed` status
	switch input {
	case "confirmed", "complete":
		return types.DepositSuccess, nil
	}
	return "", fmt.Errorf("unsupported status %s", input)
}

func toGlobalTrade(f fill) (types.Trade, error) {
	return types.Trade{
		ID:            f.TradeId,
		GID:           0,
		OrderID:       f.OrderId,
		Exchange:      types.ExchangeFTX,
		Price:         f.Price,
		Quantity:      f.Size,
		QuoteQuantity: f.Price * f.Size,
		Symbol:        toGlobalSymbol(f.Market),
		Side:          f.Side,
		IsBuyer:       f.Side == types.SideTypeBuy,
		IsMaker:       f.Liquidity == "maker",
		Time:          types.Time(f.Time.Time),
		Fee:           f.Fee,
		FeeCurrency:   f.FeeCurrency,
		IsMargin:      false,
		IsIsolated:    false,
	}, nil
}

func toGlobalKLine(symbol string, interval types.Interval, h Candle) (types.KLine, error) {
	return types.KLine{
		Exchange:  types.ExchangeFTX,
		Symbol:    toGlobalSymbol(symbol),
		StartTime: types.Time(h.StartTime.Time),
		EndTime:   types.Time(h.StartTime.Add(interval.Duration())),
		Interval:  interval,
		Open:      h.Open,
		Close:     h.Close,
		High:      h.High,
		Low:       h.Low,
		Volume:    h.Volume,
		Closed:    true,
	}, nil
}

type OrderType string

const (
	OrderTypeLimit  OrderType = "limit"
	OrderTypeMarket OrderType = "market"
)

func toLocalOrderType(orderType types.OrderType) (OrderType, bool, bool, error) {
	switch orderType {

	case types.OrderTypeLimitMaker:
		return OrderTypeLimit, true, false, nil

	case types.OrderTypeLimit:
		return OrderTypeLimit, false, false, nil

	case types.OrderTypeMarket:
		return OrderTypeMarket, false, false, nil

	case types.OrderTypeIOCLimit:
		return OrderTypeLimit, false, true, nil
	}

	return "", false, false, fmt.Errorf("order type %s not supported", orderType)
}
