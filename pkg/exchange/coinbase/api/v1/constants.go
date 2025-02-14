package coinbase

import "github.com/c9s/bbgo/pkg/types"

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

func (s *SideType) GlobalSideType() types.SideType {
	switch *s {
	case SideTypeBuy:
		return types.SideTypeBuy
	case SideTypeSell:
		return types.SideTypeSell
	}
	return types.SideTypeNone
}

type MarketType string

const (
	MarketTypeSpot MarketType = "spot"
	MarketTypeRfq  MarketType = "rfq"
)

type OrderStatus string

const (
	OrderStatusOpen     OrderStatus = "open"
	OrderStatusPending  OrderStatus = "pending"
	OrderStatusRejected OrderStatus = "rejected"
	OrderStatusDone     OrderStatus = "done"
	OrderStatusActive   OrderStatus = "active"
	OrderStatusReceived OrderStatus = "received"
	OrderStatusAll      OrderStatus = "all"
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
