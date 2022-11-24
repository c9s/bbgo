package binanceapi

import (
	"github.com/adshao/go-binance/v2"
)

type SideType = binance.SideType

const SideTypeBuy = binance.SideTypeBuy
const SideTypeSell = binance.SideTypeSell

type OrderType = binance.OrderType

const (
	OrderTypeLimit           OrderType = binance.OrderTypeLimit
	OrderTypeMarket          OrderType = binance.OrderTypeMarket
	OrderTypeLimitMaker      OrderType = binance.OrderTypeLimitMaker
	OrderTypeStopLoss        OrderType = binance.OrderTypeStopLoss
	OrderTypeStopLossLimit   OrderType = binance.OrderTypeStopLossLimit
	OrderTypeTakeProfit      OrderType = binance.OrderTypeTakeProfit
	OrderTypeTakeProfitLimit OrderType = binance.OrderTypeTakeProfitLimit
)

type OrderStatusType = binance.OrderStatusType

const (
	OrderStatusTypeNew             OrderStatusType = binance.OrderStatusTypeNew
	OrderStatusTypePartiallyFilled OrderStatusType = binance.OrderStatusTypePartiallyFilled
	OrderStatusTypeFilled          OrderStatusType = binance.OrderStatusTypeFilled
	OrderStatusTypeCanceled        OrderStatusType = binance.OrderStatusTypeCanceled
	OrderStatusTypePendingCancel   OrderStatusType = binance.OrderStatusTypePendingCancel
	OrderStatusTypeRejected        OrderStatusType = binance.OrderStatusTypeRejected
	OrderStatusTypeExpired         OrderStatusType = binance.OrderStatusTypeExpired
)

type CancelReplaceModeType string

const (
	StopOnFailure CancelReplaceModeType = "STOP_ON_FAILURE"
	AllowFailure  CancelReplaceModeType = "ALLOW_FAILURE"
)

type OrderRespType string

const (
	Ack    OrderRespType = "ACK"
	Result OrderRespType = "RESULT"
	Full   OrderRespType = "FULL"
)
