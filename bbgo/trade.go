package bbgo

import (
	"context"
	"github.com/adshao/go-binance"
	log "github.com/sirupsen/logrus"
	"strconv"
	"time"
)

type Trade struct {
	ID      int64
	Price   float64
	Volume  float64
	IsBuyer bool
	IsMaker bool
	Time    time.Time
	Symbol string
	Fee         float64
	FeeCurrency string
}

func QueryTrades(ctx context.Context, client *binance.Client, market string, startTime time.Time) (trades []Trade, err error) {
	var lastTradeID int64 = 0
	for {
		req := client.NewListTradesService().
			Limit(1000).
			Symbol(market).
			StartTime(startTime.UnixNano() / 1000000)

		if lastTradeID > 0 {
			req.FromID(lastTradeID)
		}

		bnTrades, err := req.Do(ctx)
		if err != nil {
			return nil, err
		}

		if len(bnTrades) <= 1 {
			break
		}

		for _, t := range bnTrades {
			// skip trade ID that is the same
			if t.ID == lastTradeID {
				continue
			}

			var side string
			if t.IsBuyer {
				side = "BUY"
			} else {
				side = "SELL"
			}

			// trade time
			tt := time.Unix(0, t.Time*1000000)
			log.Infof("trade: %d %4s Price: % 13s Volume: % 13s %s", t.ID, side, t.Price, t.Quantity, tt)

			price, err := strconv.ParseFloat(t.Price, 64)
			if err != nil {
				return nil, err
			}

			quantity, err := strconv.ParseFloat(t.Quantity, 64)
			if err != nil {
				return nil, err
			}

			fee, err := strconv.ParseFloat(t.Commission, 64)
			if err != nil {
				return nil, err
			}

			trades = append(trades, Trade{
				ID:          t.ID,
				Price:       price,
				Volume:      quantity,
				IsBuyer:     t.IsBuyer,
				IsMaker:     t.IsMaker,
				Fee:         fee,
				FeeCurrency: t.CommissionAsset,
				Time:        tt,
			})

			lastTradeID = t.ID
		}
	}

	return trades, nil
}
