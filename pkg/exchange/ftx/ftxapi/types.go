package ftxapi

type Liquidity string

const (
	LiquidityTaker Liquidity = "taker"
	LiquidityMaker Liquidity = "maker"
)

type Side string

const (
	SideBuy  Side = "buy"
	SideSell Side = "sell"
)

type OrderType string

const (
	OrderTypeLimit  OrderType = "limit"
	OrderTypeMarket OrderType = "market"

	// trigger order types
	OrderTypeStopLimit    OrderType = "stop"
	OrderTypeTrailingStop OrderType = "trailingStop"
	OrderTypeTakeProfit   OrderType = "takeProfit"
)

type OrderStatus string

const (
	OrderStatusNew    OrderStatus = "new"
	OrderStatusOpen   OrderStatus = "open"
	OrderStatusClosed OrderStatus = "closed"
)
