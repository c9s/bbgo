package types

import "time"

type Exchange interface {
	QueryKLines(interval string, startFrom time.Time, endTo time.Time) []KLineOrWindow
	QueryTrades(symbol string, startFrom time.Time) []Trade
}
