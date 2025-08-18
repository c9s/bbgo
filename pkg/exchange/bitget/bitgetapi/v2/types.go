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

type OrderForce string

const (
	OrderForceGTC      OrderForce = "gtc"
	OrderForcePostOnly OrderForce = "post_only"
	OrderForceFOK      OrderForce = "fok"
	OrderForceIOC      OrderForce = "ioc"
)

type OrderStatus string

const (
	OrderStatusInit          OrderStatus = "init"
	OrderStatusNew           OrderStatus = "new"
	OrderStatusLive          OrderStatus = "live"
	OrderStatusPartialFilled OrderStatus = "partially_filled"
	OrderStatusFilled        OrderStatus = "filled"
	OrderStatusCancelled     OrderStatus = "cancelled"
)

func (o OrderStatus) IsWorking() bool {
	return o == OrderStatusInit ||
		o == OrderStatusNew ||
		o == OrderStatusLive ||
		o == OrderStatusPartialFilled
}
