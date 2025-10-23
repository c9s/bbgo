package coinbase

type MarketStatus string

const (
	MarketStatusOnline   MarketStatus = "online"
	MarketStatusOffline  MarketStatus = "offline"
	MarketStatusInternal MarketStatus = "internal"
	MarketStatusDelisted MarketStatus = "delisted"
)

type Liquidity string

const (
	LiquidityMaker Liquidity = "M"
	LiquidityTaker Liquidity = "T"
	LiquidityOther Liquidity = "O"
)

type SideType string

const (
	SideTypeBuy  SideType = "buy"
	SideTypeSell SideType = "sell"
)

type MarketType string

const (
	MarketTypeSpot MarketType = "spot"
	MarketTypeRfq  MarketType = "rfq"
)

type OrderStatus string

const (
	OrderStatusReceived OrderStatus = "received"
	OrderStatusPending  OrderStatus = "pending"
	OrderStatusOpen     OrderStatus = "open"
	OrderStatusRejected OrderStatus = "rejected"
	OrderStatusDone     OrderStatus = "done"
	OrderStatusActive   OrderStatus = "active" // query only status
	OrderStatusAll      OrderStatus = "all"    // query only status

	// websocket statuses
	OrderStatusCanceled OrderStatus = "canceled"
	OrderStatusFilled   OrderStatus = "filled"
)

type OrderType string

const (
	OrderTypeLimit  OrderType = "limit"
	OrderTypeMarket OrderType = "market"
	OrderTypeStop   OrderType = "stop"
)

type TimeInForceType string

const (
	TimeInForceGTC TimeInForceType = "GTC"
	TimeInForceGTT TimeInForceType = "GTT"
	TimeInForceIOC TimeInForceType = "IOC"
	TimeInForceFOK TimeInForceType = "FOK"
)

type OrderStopType string

const (
	OrderStopTypeLoss  OrderStopType = "loss"
	OrderStopTypeEntry OrderStopType = "entry"
)
