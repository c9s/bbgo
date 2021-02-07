package types

import (
	"time"
)

type Ticker struct {
	Time   time.Time
	Volume float64 // `volume` from Max & binance
	Last   float64 // `last` from Max, `lastPrice` from binance
	Open   float64 // `open` from Max, `openPrice` from binance
	High   float64 // `high` from Max, `highPrice` from binance
	Low    float64 // `low` from Max, `lowPrice` from binance
	Buy    float64 // `buy` from Max, `bidPrice` from binance
	Sell   float64 // `sell` from Max, `askPrice` from binance
}
