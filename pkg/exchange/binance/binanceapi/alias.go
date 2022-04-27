package binanceapi

import (
	"github.com/adshao/go-binance/v2"
)

type SideType = binance.SideType

var SideTypeBuy = binance.SideTypeBuy
var SideTypeSell = binance.SideTypeSell

type OrderType = binance.OrderType

var OrderTypeLimit OrderType = binance.OrderTypeLimit
var OrderTypeMarket OrderType = binance.OrderTypeMarket
var OrderTypeLimitMaker OrderType = binance.OrderTypeLimitMaker
var OrderTypeStopLoss OrderType = binance.OrderTypeStopLoss
var OrderTypeStopLossLimit OrderType = binance.OrderTypeStopLossLimit
var OrderTypeTakeProfit OrderType = binance.OrderTypeTakeProfit
var OrderTypeTakeProfitLimit OrderType = binance.OrderTypeTakeProfitLimit

type OrderStatusType = binance.OrderStatusType

var OrderStatusTypeNew OrderStatusType = binance.OrderStatusTypeNew
var OrderStatusTypePartiallyFilled OrderStatusType = binance.OrderStatusTypePartiallyFilled
var OrderStatusTypeFilled OrderStatusType = binance.OrderStatusTypeFilled
var OrderStatusTypeCanceled OrderStatusType = binance.OrderStatusTypeCanceled
var OrderStatusTypePendingCancel OrderStatusType = binance.OrderStatusTypePendingCancel
var OrderStatusTypeRejected OrderStatusType = binance.OrderStatusTypeRejected
var OrderStatusTypeExpired OrderStatusType = binance.OrderStatusTypeExpired
