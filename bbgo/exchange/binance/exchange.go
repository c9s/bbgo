package binance

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/adshao/go-binance"

	"github.com/c9s/bbgo/pkg/bbgo/types"
	"github.com/c9s/bbgo/pkg/util"
	"github.com/sirupsen/logrus"
)

type Exchange struct {
	Client *binance.Client
}

func NewExchange(key, secret string) *Exchange {
	var client = binance.NewClient(key, secret)
	return &Exchange{
		Client: client,
	}
}

func (e *Exchange) QueryAveragePrice(ctx context.Context, symbol string) (float64, error) {
	resp, err := e.Client.NewAveragePriceService().Symbol(symbol).Do(ctx)
	if err != nil {
		return 0, err
	}

	return util.MustParseFloat(resp.Price), nil
}

func (e *Exchange) NewPrivateStream() (*PrivateStream, error) {
	return &PrivateStream{
		Client: e.Client,
	}, nil
}

func (e *Exchange) QueryAccountBalances(ctx context.Context) (map[string]types.Balance, error) {
	account, err := e.QueryAccount(ctx)
	if err != nil {
		return nil, err
	}

	return account.Balances, nil
}

func (e *Exchange) QueryAccount(ctx context.Context) (*types.Account, error) {
	account, err := e.Client.NewGetAccountService().Do(ctx)
	if err != nil {
		return nil, err
	}

	var balances = map[string]types.Balance{}
	for _, b := range account.Balances {
		balances[b.Asset] = types.Balance{
			Currency:  b.Asset,
			Available: util.MustParseFloat(b.Free),
			Locked:    util.MustParseFloat(b.Locked),
		}
	}

	return &types.Account{
		MakerCommission: account.MakerCommission,
		TakerCommission: account.TakerCommission,
		Balances:        balances,
	}, nil
}

func (e *Exchange) SubmitOrder(ctx context.Context, order *types.Order) error {
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

	orderType, err := toLocalOrderType(order.Type)
	if err != nil {
		return err
	}

	req := e.Client.NewCreateOrderService().
		Symbol(order.Symbol).
		Side(binance.SideType(order.Side)).
		Type(orderType).
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

func toLocalOrderType(orderType types.OrderType) (binance.OrderType, error) {
	switch orderType {
	case types.OrderTypeLimit:
		return binance.OrderTypeLimit, nil

	case types.OrderTypeMarket:
		return binance.OrderTypeMarket, nil
	}

	return "", fmt.Errorf("order type %s not supported", orderType)
}

func (e *Exchange) QueryKLines(ctx context.Context, symbol, interval string, options types.KLineQueryOptions) ([]types.KLine, error) {
	var limit = 500
	if options.Limit > 0 {
		// default limit == 500
		limit = options.Limit
	}

	logrus.Infof("[binance] querying kline %s %s %v", symbol, interval, options)

	req := e.Client.NewKlinesService().Symbol(symbol).
		Interval(interval).
		Limit(limit)

	if options.StartTime != nil {
		req.StartTime(options.StartTime.UnixNano() / int64(time.Millisecond))
	}

	if options.EndTime != nil {
		req.EndTime(options.EndTime.UnixNano() / int64(time.Millisecond))
	}

	resp, err := req.Do(ctx)
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

type TradeQueryOptions struct {
	StartTime   *time.Time
	EndTime     *time.Time
	Limit       int
	LastTradeID int64
}

func (e *Exchange) QueryTrades(ctx context.Context, symbol string, options *TradeQueryOptions) (trades []types.Trade, err error) {
	req := e.Client.NewListTradesService().
		Limit(1000).
		Symbol(symbol)

	if options.Limit > 0 {
		req.Limit(options.Limit)
	}

	if options.StartTime != nil {
		req.StartTime(options.StartTime.UnixNano() / int64(time.Millisecond))
	}
	if options.EndTime != nil {
		req.EndTime(options.EndTime.UnixNano() / int64(time.Millisecond))
	}
	if options.LastTradeID > 0 {
		req.FromID(options.LastTradeID)
	}

	remoteTrades, err := req.Do(ctx)
	if err != nil {
		return nil, err
	}

	for _, t := range remoteTrades {
		localTrade, err := convertRemoteTrade(*t)
		if err != nil {
			logrus.WithError(err).Errorf("can not convert binance trade: %+v", t)
			continue
		}

		logrus.Infof("[binance] trade: %d %s % 4s price: % 13s volume: % 11s %6s % 5s %s", t.ID, t.Symbol, localTrade.Side, t.Price, t.Quantity, BuyerOrSellerLabel(t), MakerOrTakerLabel(t), localTrade.Time)
		trades = append(trades, *localTrade)
	}

	return trades, nil
}

func (e *Exchange) BatchQueryTrades(ctx context.Context, symbol string, options *TradeQueryOptions) (allTrades []types.Trade, err error) {
	var startTime = time.Now().Add(-7 * 24 * time.Hour)
	if options.StartTime != nil {
		startTime = *options.StartTime
	}

	logrus.Infof("[binance] querying %s trades from %s", symbol, startTime)

	var lastTradeID = options.LastTradeID
	for {
		trades, err := e.QueryTrades(ctx, symbol, &TradeQueryOptions{
			StartTime:   &startTime,
			Limit:       options.Limit,
			LastTradeID: lastTradeID,
		})
		if err != nil {
			return allTrades, err
		}

		if len(trades) == 1 && trades[0].ID == lastTradeID {
			break
		}

		for _, t := range trades {
			// ignore the first trade if last TradeID is given
			if t.ID == lastTradeID {
				continue
			}

			allTrades = append(allTrades, t)
			lastTradeID = t.ID
		}
	}

	return allTrades, nil
}

func convertRemoteTrade(t binance.TradeV3) (*types.Trade, error) {
	// skip trade ID that is the same. however this should not happen
	var side string
	if t.IsBuyer {
		side = "BUY"
	} else {
		side = "SELL"
	}

	// trade time
	mts := time.Unix(0, t.Time*int64(time.Millisecond))

	price, err := strconv.ParseFloat(t.Price, 64)
	if err != nil {
		return nil, err
	}

	quantity, err := strconv.ParseFloat(t.Quantity, 64)
	if err != nil {
		return nil, err
	}

	quoteQuantity, err := strconv.ParseFloat(t.QuoteQuantity, 64)
	if err != nil {
		return nil, err
	}

	fee, err := strconv.ParseFloat(t.Commission, 64)
	if err != nil {
		return nil, err
	}

	return &types.Trade{
		ID:            t.ID,
		Price:         price,
		Symbol:        t.Symbol,
		Exchange:      "binance",
		Quantity:      quantity,
		Side:          side,
		IsBuyer:       t.IsBuyer,
		IsMaker:       t.IsMaker,
		Fee:           fee,
		FeeCurrency:   t.CommissionAsset,
		QuoteQuantity: quoteQuantity,
		Time:          mts,
	}, nil
}
