package bbgo

import (
	"context"

	log "github.com/sirupsen/logrus"

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
	Symbol string `json:"symbol"`

	types.IntervalWindow

	Ratio          fixedpoint.Value `json:"ratio"`
	MinQuoteVolume fixedpoint.Value `json:"minQuoteVolume"`

	session       *ExchangeSession
	orderExecutor *GeneralOrderExecutor
}

func (s *CumulatedVolumeTakeProfit) Bind(session *ExchangeSession, orderExecutor *GeneralOrderExecutor) {
	s.session = session
	s.orderExecutor = orderExecutor

	position := orderExecutor.Position()

	store, _ := session.MarketDataStore(position.Symbol)

	session.MarketDataStream.OnKLineClosed(types.KLineWith(s.Symbol, s.Interval, func(kline types.KLine) {
		closePrice := kline.Close
		if position.IsClosed() || position.IsDust(closePrice) {
			return
		}

		roi := position.ROI(closePrice)
		if roi.Sign() < 0 {
			return
		}

		klines, ok := store.KLinesOfInterval(s.Interval)
		if !ok {
			log.Warnf("history kline not found")
			return
		}

		if len(*klines) < s.Window {
			return
		}

		var cbv = fixedpoint.Zero
		var cqv = fixedpoint.Zero
		for i := 0; i < s.Window; i++ {
			last := (*klines)[len(*klines)-1-i]
			cqv = cqv.Add(last.QuoteVolume)
			cbv = cbv.Add(last.Volume)
		}

		if cqv.Compare(s.MinQuoteVolume) > 0 {
			Notify("[CumulatedVolumeTakeProfit] %s TakeProfit triggered by cumulated volume (window: %d) %f > %f, price = %f",
				position.Symbol,
				s.Window,
				cqv.Float64(),
				s.MinQuoteVolume.Float64(), kline.Close.Float64())

			if err := orderExecutor.ClosePosition(context.Background(), fixedpoint.One, "cumulatedVolumeTakeProfit"); err != nil {
				log.WithError(err).Errorf("close position error")
			}
			return
		}
	}))
}
