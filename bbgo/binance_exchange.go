package bbgo

import (
	"context"
	"github.com/adshao/go-binance"
	log "github.com/sirupsen/logrus"
	"strconv"
	"time"
)

type BinanceExchange struct {
	Client *binance.Client
}

func (e *BinanceExchange) SubmitOrder(ctx context.Context, order Order) error {
	/*
		limit order example

			order, err := Client.NewCreateOrderService().
				Symbol(Symbol).
				Side(side).
				Type(binance.OrderTypeLimit).
				TimeInForce(binance.TimeInForceTypeGTC).
				Quantity(volumeString).
				Price(priceString).
				Do(ctx)
	*/

	req := e.Client.NewCreateOrderService().
		Symbol(order.Symbol).
		Side(order.Side).
		Type(order.Type).
		Quantity(order.VolumeStr)

	if len(order.PriceStr) > 0 {
		req.Price(order.PriceStr)
	}
	if len(order.TimeInForce) > 0 {
		req.TimeInForce(order.TimeInForce)
	}

	retOrder, err := req.Do(ctx)
	log.Infof("order created: %+v", retOrder)
	return err
}

func (e *BinanceExchange) QueryKLines(ctx context.Context, symbol, interval string, limit int) ([]KLine, error) {
	resp, err := e.Client.NewKlinesService().Symbol(symbol).Interval(interval).Limit(limit).Do(ctx)
	if err != nil {
		return nil, err
	}

	var kLines []KLine
	for _, kline := range resp {
		kLines = append(kLines, KLine{
			Symbol:         symbol,
			Interval:       interval,
			StartTime:      kline.OpenTime,
			EndTime:        kline.CloseTime,
			Open:           kline.Open,
			Close:          kline.Close,
			High:           kline.High,
			Low:            kline.Low,
			Volume:         kline.Volume,
			QuoteVolume:    kline.QuoteAssetVolume,
			NumberOfTrades: kline.TradeNum,
		})
	}
	return kLines, nil
}

func (e *BinanceExchange) QueryTrades(ctx context.Context, market string, startTime time.Time) (trades []Trade, err error) {
	var lastTradeID int64 = 0
	for {
		req := e.Client.NewListTradesService().
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

