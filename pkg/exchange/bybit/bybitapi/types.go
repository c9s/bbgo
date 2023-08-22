package bybitapi

import "github.com/c9s/bbgo/pkg/types"

var (
	SupportedIntervals = map[types.Interval]int{
		types.Interval1m:  1 * 60,
		types.Interval3m:  3 * 60,
		types.Interval5m:  5 * 60,
		types.Interval15m: 15 * 60,
		types.Interval30m: 30 * 60,
		types.Interval1h:  60 * 60,
		types.Interval2h:  60 * 60 * 2,
		types.Interval4h:  60 * 60 * 4,
		types.Interval6h:  60 * 60 * 6,
		types.Interval12h: 60 * 60 * 12,
		types.Interval1d:  60 * 60 * 24,
		types.Interval1w:  60 * 60 * 24 * 7,
		types.Interval1mo: 60 * 60 * 24 * 30,
	}

	ToGlobalInterval = map[string]types.Interval{
		"1":   types.Interval1m,
		"3":   types.Interval3m,
		"5":   types.Interval5m,
		"15":  types.Interval15m,
		"30":  types.Interval30m,
		"60":  types.Interval1h,
		"120": types.Interval2h,
		"240": types.Interval4h,
		"360": types.Interval6h,
		"720": types.Interval12h,
		"D":   types.Interval1d,
		"W":   types.Interval1w,
		"M":   types.Interval1mo,
	}
)

type Category string

const (
	CategorySpot Category = "spot"
)

type Status string

const (
	// StatusTrading is only include the "Trading" status for `spot` category.
	StatusTrading Status = "Trading"
)

type OpenOnly int

const (
	OpenOnlyOrder OpenOnly = 0
)

type Side string

const (
	SideBuy  Side = "Buy"
	SideSell Side = "Sell"
)

type OrderStatus string

const (
	// OrderStatusCreated order has been accepted by the system but not yet put through the matching engine
	OrderStatusCreated OrderStatus = "Created"
	// OrderStatusNew is order has been placed successfully.
	OrderStatusNew             OrderStatus = "New"
	OrderStatusRejected        OrderStatus = "Rejected"
	OrderStatusPartiallyFilled OrderStatus = "PartiallyFilled"
	// OrderStatusPartiallyFilledCanceled means that the order has been partially filled but not all then cancel.
	OrderStatusPartiallyFilledCanceled OrderStatus = "PartiallyFilledCanceled"
	OrderStatusFilled                  OrderStatus = "Filled"
	OrderStatusCancelled               OrderStatus = "Cancelled"

	// Following statuses is conditional orders. Once you place conditional orders, it will be in untriggered status.
	// Untriggered -> Triggered ->  New
	// Once the trigger price reached, order status will be moved to triggered
	// Singe BBGO not support Untriggered/Triggered, so comment it.
	//
	// OrderStatusUntriggered means that the order not triggered
	//OrderStatusUntriggered OrderStatus = "Untriggered"
	//// OrderStatusTriggered means that the order has been triggered
	//OrderStatusTriggered OrderStatus = "Triggered"

	// Following statuses is stop orders
	// OrderStatusDeactivated is an order status for stopOrders.
	//e.g. when you place a conditional order, then you cancel it, this order status is "Deactivated"
	OrderStatusDeactivated OrderStatus = "Deactivated"

	// OrderStatusActive order has been triggered and the new active order has been successfully placed. Is the final
	// state of a successful conditional order
	OrderStatusActive OrderStatus = "Active"
)

var (
	AllOrderStatuses = []OrderStatus{
		OrderStatusCreated,
		OrderStatusNew,
		OrderStatusRejected,
		OrderStatusPartiallyFilled,
		OrderStatusPartiallyFilledCanceled,
		OrderStatusFilled,
		OrderStatusCancelled,
		OrderStatusDeactivated,
		OrderStatusActive,
	}
)

type OrderType string

const (
	OrderTypeMarket OrderType = "Market"
	OrderTypeLimit  OrderType = "Limit"
)

type TimeInForce string

const (
	TimeInForceGTC TimeInForce = "GTC"
	TimeInForceIOC TimeInForce = "IOC"
	TimeInForceFOK TimeInForce = "FOK"
)

type AccountType string

const AccountTypeSpot AccountType = "SPOT"
