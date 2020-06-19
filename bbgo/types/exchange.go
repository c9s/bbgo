package types

import "time"

type Trade interface {
	GetPrice() float64
	GetVolume() float64
	GetFeeCurrency() string
	GetFeeAmount() float64
}

type Exchange interface {
	QueryKLines(interval string, startFrom time.Time, endTo time.Time) []KLine
	QueryTrades(symbol string, startFrom time.Time) []Trade
}
