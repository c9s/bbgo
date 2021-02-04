package types

import (
	"time"
)

type Ticker struct {
	Time   time.Time
	Volume string // `volume` from Max & binance
	Last   string // `last` from Max, `lastPrice` from binance
	Open   string // `open` from Max, `openPrice` from binance
	High   string // `high` from Max, `highPrice` from binance
	Low    string // `low` from Max, `lowPrice` from binance
	Buy    string // `buy` from Max, `bidPrice` from binance
	Sell   string // `sell` from Max, `askPrice` from binance
}
