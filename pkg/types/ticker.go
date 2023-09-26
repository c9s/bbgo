package types

import (
	"fmt"
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

type Ticker struct {
	Time   time.Time
	Volume fixedpoint.Value // `volume` from Max & binance
	Last   fixedpoint.Value // `last` from Max, `lastPrice` from binance
	Open   fixedpoint.Value // `open` from Max, `openPrice` from binance
	High   fixedpoint.Value // `high` from Max, `highPrice` from binance
	Low    fixedpoint.Value // `low` from Max, `lowPrice` from binance
	Buy    fixedpoint.Value // `buy` from Max, `bidPrice` from binance
	Sell   fixedpoint.Value // `sell` from Max, `askPrice` from binance
}

func (t *Ticker) String() string {
	return fmt.Sprintf("O:%s H:%s L:%s LAST:%s BID/ASK:%s/%s TIME:%s", t.Open, t.High, t.Low, t.Last, t.Buy, t.Sell, t.Time.String())
}
