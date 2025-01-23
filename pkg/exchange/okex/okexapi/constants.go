package okexapi

// TradeMode: see https://www.okx.com/docs-v5/trick_en/#order-management-trade-mode
type TradeMode string

const (
	TradeModeCash     TradeMode = "cash"
	TradeModeIsolated TradeMode = "isolated"
	TradeModeCross    TradeMode = "cross"
)

type TargetCurrency string

const (
	TargetCurrencyBase  TargetCurrency = "base_ccy"
	TargetCurrencyQuote TargetCurrency = "quote_ccy"
)

type MarginMode string

const (
	MarginModeIsolated = "isolated"
	MarginModeCross    = "cross"
)

type LiquidityType string

const (
	LiquidityTypeMaker = "M"
	LiquidityTypeTaker = "T"
)

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
	InstrumentTypeMargin  InstrumentType = "MARGIN"
)

type OrderState string

const (
	OrderStateCanceled        OrderState = "canceled"
	OrderStateLive            OrderState = "live"
	OrderStatePartiallyFilled OrderState = "partially_filled"
	OrderStateFilled          OrderState = "filled"
)
