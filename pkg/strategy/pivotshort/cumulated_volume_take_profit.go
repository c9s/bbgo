package pivotshort

import (
	"context"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

// CumulatedVolumeTakeProfit
// This exit method cumulate the volume by N bars, if the cumulated volume exceeded a threshold, then we take profit.
//
// To query the historical quote volume, use the following query:
//
// > SELECT start_time, `interval`, quote_volume, open, close FROM binance_klines WHERE symbol = 'ETHUSDT' AND `interval` = '5m' ORDER BY quote_volume DESC LIMIT 20;
//
type CumulatedVolumeTakeProfit struct {
	types.IntervalWindow
	Ratio          fixedpoint.Value `json:"ratio"`
	MinQuoteVolume fixedpoint.Value `json:"minQuoteVolume"`

	session       *bbgo.ExchangeSession
	orderExecutor *bbgo.GeneralOrderExecutor
}

func (s *CumulatedVolumeTakeProfit) Bind(session *bbgo.ExchangeSession, orderExecutor *bbgo.GeneralOrderExecutor) {
	s.session = session
	s.orderExecutor = orderExecutor

	position := orderExecutor.Position()

	store, _ := session.MarketDataStore(position.Symbol)

	session.MarketDataStream.OnKLineClosed(func(kline types.KLine) {
		if kline.Symbol != position.Symbol || kline.Interval != types.Interval1m {
			return
		}

		closePrice := kline.Close
		if position.IsClosed() || position.IsDust(closePrice) {
			return
		}

		roi := position.ROI(closePrice)
		if roi.Sign() < 0 {
			return
		}

		if klines, ok := store.KLinesOfInterval(s.Interval); ok {
			var cbv = fixedpoint.Zero
			var cqv = fixedpoint.Zero
			for i := 0; i < s.Window; i++ {
				last := (*klines)[len(*klines)-1-i]
				cqv = cqv.Add(last.QuoteVolume)
				cbv = cbv.Add(last.Volume)
			}

			if cqv.Compare(s.MinQuoteVolume) > 0 {
				bbgo.Notify("%s TakeProfit triggered by cumulated volume (window: %d) %f > %f, price = %f",
					position.Symbol,
					s.Window,
					cqv.Float64(),
					s.MinQuoteVolume.Float64(), kline.Close.Float64())

				_ = orderExecutor.ClosePosition(context.Background(), fixedpoint.One, "cumulatedVolumeTakeProfit")
				return
			}
		}
	})
}
