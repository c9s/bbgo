package bfxapi

//go:generate mapgen -type OrderFlag
type OrderFlag int

const (
	OrderFlagHidden     OrderFlag = 64    // hidden order
	OrderFlagClose      OrderFlag = 512   // close position
	OrderFlagReduceOnly OrderFlag = 1024  // reduce-only
	OrderFlagPostOnly   OrderFlag = 4096  // post-only order
	OrderFlagOCO        OrderFlag = 16384 // one-cancels-other
	OrderFlagNoVarRate  OrderFlag = 524288
)

//go:generate mapgen -type OrderStatus
type OrderStatus string

const (
	OrderStatusActive            OrderStatus = "ACTIVE"             // order is active
	OrderStatusExecuted          OrderStatus = "EXECUTED"           // order has been fully filled
	OrderStatusPartiallyFilled   OrderStatus = "PARTIALLY FILLED"   // order has been partially filled
	OrderStatusCanceled          OrderStatus = "CANCELED"           // order has been canceled
	OrderStatusPostponed         OrderStatus = "POSTPONED"          // order has been postponed
	OrderStatusInsufficientBal   OrderStatus = "INSUFFICIENT BAL"   // insufficient balance
	OrderStatusNotEnoughBalance  OrderStatus = "NOT ENOUGH BALANCE" // not enough balance
	OrderStatusNotFound          OrderStatus = "NOT FOUND"          // order not found
	OrderStatusStopped           OrderStatus = "STOPPED"            // order stopped
	OrderStatusRejected          OrderStatus = "REJECTED"           // order rejected
	OrderStatusExpired           OrderStatus = "EXPIRED"            // order expired
	OrderStatusPending           OrderStatus = "PENDING"            // order pending
	OrderStatusPartiallyCanceled OrderStatus = "PARTIALLY CANCELED" // order partially canceled
)

//go:generate mapgen -type OrderType
type OrderType string

const (
	OrderTypeLimit                OrderType = "LIMIT"                  // limit order
	OrderTypeExchangeLimit        OrderType = "EXCHANGE LIMIT"         // exchange limit order
	OrderTypeMarket               OrderType = "MARKET"                 // market order
	OrderTypeExchangeMarket       OrderType = "EXCHANGE MARKET"        // exchange market order
	OrderTypeStop                 OrderType = "STOP"                   // stop order
	OrderTypeExchangeStop         OrderType = "EXCHANGE STOP"          // exchange stop order
	OrderTypeStopLimit            OrderType = "STOP LIMIT"             // stop limit order
	OrderTypeExchangeStopLimit    OrderType = "EXCHANGE STOP LIMIT"    // exchange stop limit order
	OrderTypeTrailingStop         OrderType = "TRAILING STOP"          // trailing stop order
	OrderTypeExchangeTrailingStop OrderType = "EXCHANGE TRAILING STOP" // exchange trailing stop order
	OrderTypeFOK                  OrderType = "FOK"                    // fill-or-kill order
	OrderTypeExchangeFOK          OrderType = "EXCHANGE FOK"           // exchange fill-or-kill order
	OrderTypeIOC                  OrderType = "IOC"                    // immediate-or-cancel order
	OrderTypeExchangeIOC          OrderType = "EXCHANGE IOC"           // exchange immediate-or-cancel order
)
