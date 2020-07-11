package binance

import (
	"context"
	"github.com/adshao/go-binance"
	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/sirupsen/logrus"
	"strconv"
	"time"
)

type Exchange struct {
	Client *binance.Client
}

func (e *Exchange) QueryAveragePrice(ctx context.Context, symbol string) (float64, error) {
	resp, err := e.Client.NewAveragePriceService().Symbol(symbol).Do(ctx)
	if err != nil {
		return 0, err
	}

	return bbgo.MustParseFloat(resp.Price), nil
}

func (e *Exchange) NewPrivateStream(ctx context.Context) (*PrivateStream, error) {
	logrus.Infof("[binance] creating user data stream...")
	listenKey, err := e.Client.NewStartUserStreamService().Do(ctx)
	if err != nil {
		return nil, err
	}

	logrus.Infof("[binance] user data stream created. listenKey: %s", listenKey)
	return &PrivateStream{
		Client:    e.Client,
		ListenKey: listenKey,
	}, nil
}

func (e *Exchange) SubmitOrder(ctx context.Context, order *bbgo.Order) error {
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
	logrus.Infof("[binance] order created: %+v", retOrder)
	return err
}

func (e *Exchange) QueryKLines(ctx context.Context, symbol, interval string, limit int) ([]types.KLine, error) {
	logrus.Infof("[binance] querying kline %s %s limit %d", symbol, interval, limit)

	resp, err := e.Client.NewKlinesService().Symbol(symbol).Interval(interval).Limit(limit).Do(ctx)
	if err != nil {
		return nil, err
	}

	var kLines []types.KLine
	for _, kline := range resp {
		kLines = append(kLines, types.KLine{
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

func (e *Exchange) QueryTrades(ctx context.Context, symbol string, startTime time.Time) (trades []types.Trade, err error) {
	logrus.Infof("[binance] querying %s trades from %s", symbol, startTime)

	var lastTradeID int64 = 0
	for {
		req := e.Client.NewListTradesService().
			Limit(1000).
			Symbol(symbol).
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
			// skip trade ID that is the same. however this should not happen
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

			logrus.Infof("[binance] trade: %d %s % 4s price: % 13s volume: % 11s %6s % 5s %s", t.ID, t.Symbol, side, t.Price, t.Quantity, BuyerOrSellerLabel(t), MakerOrTakerLabel(t), tt)

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

			trades = append(trades, types.Trade{
				ID:          t.ID,
				Price:       price,
				Volume:      quantity,
				Side:        side,
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
