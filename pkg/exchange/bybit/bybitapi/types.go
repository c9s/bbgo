package bybitapi

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
	OrderStatusNew                     OrderStatus = "New"
	OrderStatusRejected                OrderStatus = "Rejected"
	OrderStatusPartiallyFilled         OrderStatus = "PartiallyFilled"
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
