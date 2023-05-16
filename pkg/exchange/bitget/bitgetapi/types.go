package bitgetapi

type SideType string

const (
	SideTypeBuy  SideType = "buy"
	SideTypeSell SideType = "sell"
)

type OrderType string

const (
	OrderTypeMarket   OrderType = "market"
	OrderTypeLimit    OrderType = "limit"
	OrderTypePostOnly OrderType = "post_only"
	OrderTypeFOK      OrderType = "fok"
	OrderTypeIOC      OrderType = "ioc"
)

type InstrumentType string

const (
	InstrumentTypeSpot    InstrumentType = "SPOT"
	InstrumentTypeSwap    InstrumentType = "SWAP"
	InstrumentTypeFutures InstrumentType = "FUTURES"
	InstrumentTypeOption  InstrumentType = "OPTION"
)

type OrderState string

const (
	OrderStateCanceled        OrderState = "canceled"
	OrderStateLive            OrderState = "live"
	OrderStatePartiallyFilled OrderState = "partially_filled"
	OrderStateFilled          OrderState = "filled"
)
