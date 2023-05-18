package bitgetapi

type SideType string

const (
	SideTypeBuy  SideType = "buy"
	SideTypeSell SideType = "sell"
)

type OrderType string

const (
	OrderTypeLimit  OrderType = "limit"
	OrderTypeMarket OrderType = "market"
)

type OrderSide string

const (
	OrderSideBuy  OrderSide = "buy"
	OrderSideSell OrderSide = "sell"
)

type OrderForce string

const (
	OrderForceGTC      OrderForce = "normal"
	OrderForcePostOnly OrderForce = "post_only"
	OrderForceFOK      OrderForce = "fok"
	OrderForceIOC      OrderForce = "ioc"
)
