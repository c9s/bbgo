package kucoinapi

type AccountType string

const (
	AccountTypeMain   AccountType = "main"
	AccountTypeTrade  AccountType = "trade"
	AccountTypeMargin AccountType = "margin"
	AccountTypePool   AccountType = "pool"
)

type TradeType string

const (
	TradeTypeSpot   TradeType = "TRADE"
	TradeTypeMargin TradeType = "MARGIN"
)

type SideType string

const (
	SideTypeBuy  SideType = "buy"
	SideTypeSell SideType = "sell"
)

type TimeInForceType string

const (
	// GTC Good Till Canceled orders remain open on the book until canceled. This is the default behavior if no policy is specified.
	TimeInForceGTC TimeInForceType = "GTC"

	// GTT Good Till Time orders remain open on the book until canceled or the allotted cancelAfter is depleted on the matching engine. GTT orders are guaranteed to cancel before any other order is processed after the cancelAfter seconds placed in order book.
	TimeInForceGTT TimeInForceType = "GTT"

	// FOK Fill Or Kill orders are rejected if the entire size cannot be matched.
	TimeInForceFOK TimeInForceType = "FOK"

	// IOC Immediate Or Cancel orders instantly cancel the remaining size of the limit order instead of opening it on the book.
	TimeInForceIOC TimeInForceType = "IOC"
)

type LiquidityType string

const (
	LiquidityTypeMaker LiquidityType = "maker"
	LiquidityTypeTaker LiquidityType = "taker"
)

type OrderType string

const (
	OrderTypeMarket    OrderType = "market"
	OrderTypeLimit     OrderType = "limit"
	OrderTypeStopLimit OrderType = "stop_limit"
)

type OrderState string

const (
	OrderStateCanceled        OrderState = "canceled"
	OrderStateLive            OrderState = "live"
	OrderStatePartiallyFilled OrderState = "partially_filled"
	OrderStateFilled          OrderState = "filled"
)
