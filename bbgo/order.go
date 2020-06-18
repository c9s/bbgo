package bbgo

import "github.com/adshao/go-binance"

type Order struct {
	Symbol    string
	Side      binance.SideType
	Type      binance.OrderType
	VolumeStr string
	PriceStr  string

	TimeInForce binance.TimeInForceType
}

