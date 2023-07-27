package bybit

import (
	"fmt"
	"hash/fnv"
	"math"
	"time"

	"github.com/c9s/bbgo/pkg/exchange/bybit/bybitapi"
	"github.com/c9s/bbgo/pkg/types"
)

func toGlobalMarket(m bybitapi.Instrument) types.Market {
	return types.Market{
		Symbol:          m.Symbol,
		LocalSymbol:     m.Symbol,
		PricePrecision:  int(math.Log10(m.LotSizeFilter.QuotePrecision.Float64())),
		VolumePrecision: int(math.Log10(m.LotSizeFilter.BasePrecision.Float64())),
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
	status, err := toGlobalOrderStatus(order.OrderStatus)
	if err != nil {
		return nil, err
	}
	working, err := isWorking(order.OrderStatus)
	if err != nil {
		return nil, err
	}

	return &types.Order{
		SubmitOrder: types.SubmitOrder{
			ClientOrderID: order.OrderLinkId,
			Symbol:        order.Symbol,
			Side:          side,
			Type:          orderType,
			Quantity:      order.Qty,
			Price:         order.Price,
			TimeInForce:   timeInForce,
		},
		Exchange:         types.ExchangeBybit,
		OrderID:          hashStringID(order.OrderId),
		UUID:             order.OrderId,
		Status:           status,
		ExecutedQuantity: order.CumExecQty,
		IsWorking:        working,
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

func toGlobalOrderStatus(status bybitapi.OrderStatus) (types.OrderStatus, error) {
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
		bybitapi.OrderStatusPartiallyFilledCanceled,
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

func hashStringID(s string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(s))
	return h.Sum64()
}

func isWorking(status bybitapi.OrderStatus) (bool, error) {
	s, err := toGlobalOrderStatus(status)
	return s == types.OrderStatusNew || s == types.OrderStatusPartiallyFilled, err
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
