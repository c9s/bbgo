package binance

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/adshao/go-binance"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo/types"
	"github.com/c9s/bbgo/pkg/util"
)

var log = logrus.WithFields(logrus.Fields{
	"exchange": "binance",
})

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

type Withdraw struct {
	ID         string  `json:"id"`
	Asset      string  `json:"asset"`
	Amount     float64 `json:"amount"`
	Address    string  `json:"address"`
	AddressTag string  `json:"addressTag"`
	Status     string  `json:"status"`

	TransactionID   string    `json:"txId"`
	TransactionFee  float64   `json:"transactionFee"`
	WithdrawOrderID string    `json:"withdrawOrderId"`
	ApplyTime       time.Time `json:"applyTime"`
	Network         string    `json:"network"`
}

func (e *Exchange) QueryWithdrawHistory(ctx context.Context, asset string, since, until time.Time) (allWithdraws []Withdraw, err error) {

	startTime := since
	txIDs := map[string]struct{}{}

	for startTime.Before(until) {
		// startTime ~ endTime must be in 90 days
		endTime := startTime.AddDate(0, 0, 60)
		if endTime.After(until) {
			endTime = until
		}

		withdraws, err := e.Client.NewListWithdrawsService().
			Asset(asset).
			StartTime(startTime.UnixNano() / int64(time.Millisecond)).
			EndTime(endTime.UnixNano() / int64(time.Millisecond)).
			Do(ctx)

		if err != nil {
			return nil, err
		}

		for _, d := range withdraws {
			if _, ok := txIDs[d.TxID]; ok {
				continue
			}

			// 0(0:pending,6: credited but cannot withdraw, 1:success)
			status := ""
			switch d.Status {
			case 0:
				status = "email_sent"
			case 1:
				status = "cancelled"
			case 2:
				status = "awaiting_approval"
			case 3:
				status = "rejected"
			case 4:
				status = "processing"
			case 5:
				status = "failure"
			case 6:
				status = "completed"
			}

			txIDs[d.TxID] = struct{}{}
			allWithdraws = append(allWithdraws, Withdraw{
				ApplyTime:       time.Unix(0, d.ApplyTime*int64(time.Millisecond)),
				Asset:           d.Asset,
				Amount:          d.Amount,
				Address:         d.Address,
				AddressTag:      d.AddressTag,
				TransactionID:   d.TxID,
				TransactionFee:  d.TransactionFee,
				WithdrawOrderID: d.WithdrawOrderID,
				Network:         d.Network,
				Status:          status,
			})
		}

		startTime = endTime
	}

	return allWithdraws, nil
}

type Deposit struct {
	Time          time.Time `json:"time"`
	Amount        float64   `json:"amount"`
	Asset         string    `json:"asset"`
	Address       string    `json:"address"`
	AddressTag    string    `json:"addressTag"`
	TransactionID string    `json:"txId"`
	Status        string    `json:"status"`
}

func (e *Exchange) QueryDepositHistory(ctx context.Context, asset string, since, until time.Time) (allDeposits []Deposit, err error) {
	startTime := since
	txIDs := map[string]struct{}{}
	for startTime.Before(until) {

		// startTime ~ endTime must be in 90 days
		endTime := startTime.AddDate(0, 0, 60)
		if endTime.After(until) {
			endTime = until
		}

		deposits, err := e.Client.NewListDepositsService().
			Asset(asset).
			StartTime(startTime.UnixNano() / int64(time.Millisecond)).
			EndTime(endTime.UnixNano() / int64(time.Millisecond)).
			Do(ctx)

		if err != nil {
			return nil, err
		}

		for _, d := range deposits {
			if _, ok := txIDs[d.TxID]; ok {
				continue
			}

			// 0(0:pending,6: credited but cannot withdraw, 1:success)
			status := ""
			switch d.Status {
			case 0:
				status = "pending"
			case 6:
				status = "credited"
			case 1:
				status = "success"
			}

			txIDs[d.TxID] = struct{}{}
			allDeposits = append(allDeposits, Deposit{
				Time:          time.Unix(0, d.InsertTime*int64(time.Millisecond)),
				Asset:         d.Asset,
				Amount:        d.Amount,
				Address:       d.Address,
				AddressTag:    d.AddressTag,
				TransactionID: d.TxID,
				Status:        status,
			})
		}

		startTime = endTime
	}

	return allDeposits, nil
}

func (e *Exchange) QueryAccountBalances(ctx context.Context) (map[string]types.Balance, error) {
	account, err := e.QueryAccount(ctx)
	if err != nil {
		return nil, err
	}

	return account.Balances, nil
}

// TradingFeeCurrency
func (e *Exchange) TradingFeeCurrency() string {
	return "BNB"
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

func (e *Exchange) SubmitOrder(ctx context.Context, order *types.SubmitOrder) error {
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
		Quantity(order.Quantity)

	if len(order.Price) > 0 {
		req.Price(order.Price)
	}
	if len(order.TimeInForce) > 0 {
		req.TimeInForce(order.TimeInForce)
	}

	retOrder, err := req.Do(ctx)
	log.Infof("order created: %+v", retOrder)
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

	log.Infof("querying kline %s %s %v", symbol, interval, options)

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
			log.WithError(err).Errorf("can not convert binance trade: %+v", t)
			continue
		}

		log.Infof("trade: %d %s % 4s price: % 13s volume: % 11s %6s % 5s %s", t.ID, t.Symbol, localTrade.Side, t.Price, t.Quantity, BuyerOrSellerLabel(t), MakerOrTakerLabel(t), localTrade.Time)
		trades = append(trades, *localTrade)
	}

	return trades, nil
}

func (e *Exchange) BatchQueryTrades(ctx context.Context, symbol string, options *TradeQueryOptions) (allTrades []types.Trade, err error) {
	var startTime = time.Now().Add(-7 * 24 * time.Hour)
	if options.StartTime != nil {
		startTime = *options.StartTime
	}

	log.Infof("querying %s trades from %s", symbol, startTime)

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

func (e *Exchange) BatchQueryKLines(ctx context.Context, symbol, interval string, startTime, endTime time.Time) ([]types.KLine, error) {
	var allKLines []types.KLine

	for startTime.Before(endTime) {
		klines, err := e.QueryKLines(ctx, symbol, interval, types.KLineQueryOptions{
			StartTime: &startTime,
			Limit:     1000,
		})

		if err != nil {
			return nil, err
		}

		for _, kline := range klines {
			t := time.Unix(0, kline.EndTime*int64(time.Millisecond))
			if t.After(endTime) {
				return allKLines, nil
			}

			allKLines = append(allKLines, kline)
			startTime = t
		}

		// avoid rate limit
		time.Sleep(100 * time.Millisecond)
	}

	return allKLines, nil
}

func (e *Exchange) BatchQueryKLineWindows(ctx context.Context, symbol string, intervals []string, startTime, endTime time.Time) (map[string]types.KLineWindow, error) {
	klineWindows := map[string]types.KLineWindow{}
	for _, interval := range intervals {
		klines, err := e.BatchQueryKLines(ctx, symbol, interval, startTime, endTime)
		if err != nil {
			return klineWindows, err
		}
		klineWindows[interval] = klines
	}

	return klineWindows, nil
}
