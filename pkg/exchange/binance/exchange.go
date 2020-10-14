package binance

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/adshao/go-binance"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
)

var log = logrus.WithFields(logrus.Fields{
	"exchange": "binance",
})

func init() {
	_ = types.Exchange(&Exchange{})
}

type Exchange struct {
	Client *binance.Client
}

func New(key, secret string) *Exchange {
	var client = binance.NewClient(key, secret)
	return &Exchange{
		Client: client,
	}
}

func (e *Exchange) Name() types.ExchangeName {
	return types.ExchangeBinance
}

func (e *Exchange) QueryMarkets(ctx context.Context) (types.MarketMap, error) {
	exchangeInfo, err := e.Client.NewExchangeInfoService().Do(ctx)
	if err != nil {
		return nil, err
	}

	markets := types.MarketMap{}
	for _, symbol := range exchangeInfo.Symbols {
		market := types.Market{
			Symbol:          symbol.Symbol,
			PricePrecision:  symbol.QuotePrecision,
			VolumePrecision: symbol.BaseAssetPrecision,
			QuoteCurrency:   symbol.QuoteAsset,
			BaseCurrency:    symbol.BaseAsset,
			MinAmount:       0,
			MinNotional:     0,
			MinLot:          0,
		}

		if f := symbol.MinNotionalFilter() ; f != nil {
			market.MinNotional = util.MustParseFloat(f.MinNotional)
		}

		// The LOT_SIZE filter defines the quantity (aka "lots" in auction terms) rules for a symbol.
		// There are 3 parts:
		// minQty defines the minimum quantity/icebergQty allowed.
		//	maxQty defines the maximum quantity/icebergQty allowed.
		//	stepSize defines the intervals that a quantity/icebergQty can be increased/decreased by.
		if f := symbol.LotSizeFilter() ; f != nil {
			market.MinLot = util.MustParseFloat(f.MinQuantity)
			market.MinQuantity = util.MustParseFloat(f.MinQuantity)
			market.MaxQuantity = util.MustParseFloat(f.MaxQuantity)
			// market.StepSize = util.MustParseFloat(f.StepSize)
		}

		if f := symbol.PriceFilter() ; f != nil {
			market.MaxPrice = util.MustParseFloat(f.MaxPrice)
			market.MinPrice = util.MustParseFloat(f.MinPrice)
			market.TickSize = util.MustParseFloat(f.TickSize)
		}

		markets[symbol.Symbol] = market
	}

	return markets, nil
}

func (e *Exchange) QueryAveragePrice(ctx context.Context, symbol string) (float64, error) {
	resp, err := e.Client.NewAveragePriceService().Symbol(symbol).Do(ctx)
	if err != nil {
		return 0, err
	}

	return util.MustParseFloat(resp.Price), nil
}

func (e *Exchange) NewStream() types.Stream {
	return NewStream(e.Client)
}

func (e *Exchange) QueryWithdrawHistory(ctx context.Context, asset string, since, until time.Time) (allWithdraws []types.Withdraw, err error) {

	startTime := since
	txIDs := map[string]struct{}{}

	for startTime.Before(until) {
		// startTime ~ endTime must be in 90 days
		endTime := startTime.AddDate(0, 0, 60)
		if endTime.After(until) {
			endTime = until
		}

		req := e.Client.NewListWithdrawsService()
		if len(asset) > 0 {
			req.Asset(asset)
		}

		withdraws, err := req.
			StartTime(startTime.UnixNano() / int64(time.Millisecond)).
			EndTime(endTime.UnixNano() / int64(time.Millisecond)).
			Do(ctx)

		if err != nil {
			return allWithdraws, err
		}

		for _, d := range withdraws {
			if _, ok := txIDs[d.TxID]; ok {
				continue
			}

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

			default:
				status = fmt.Sprintf("unsupported code: %d", d.Status)
			}

			txIDs[d.TxID] = struct{}{}
			allWithdraws = append(allWithdraws, types.Withdraw{
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

func (e *Exchange) QueryDepositHistory(ctx context.Context, asset string, since, until time.Time) (allDeposits []types.Deposit, err error) {
	startTime := since
	txIDs := map[string]struct{}{}
	for startTime.Before(until) {

		// startTime ~ endTime must be in 90 days
		endTime := startTime.AddDate(0, 0, 60)
		if endTime.After(until) {
			endTime = until
		}

		req := e.Client.NewListDepositsService()
		if len(asset) > 0 {
			req.Asset(asset)
		}

		deposits, err := req.
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
			status := types.DepositStatus(fmt.Sprintf("code: %d", d.Status))

			switch d.Status {
			case 0:
				status = types.DepositPending
			case 6:
				// https://www.binance.com/en/support/faq/115003736451
				status = types.DepositCredited
			case 1:
				status = types.DepositSuccess
			}

			txIDs[d.TxID] = struct{}{}
			allDeposits = append(allDeposits, types.Deposit{
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

func (e *Exchange) QueryAccountBalances(ctx context.Context) (types.BalanceMap, error) {
	account, err := e.QueryAccount(ctx)
	if err != nil {
		return nil, err
	}

	return account.Balances, nil
}

// PlatformFeeCurrency
func (e *Exchange) PlatformFeeCurrency() string {
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

func (e *Exchange) SubmitOrder(ctx context.Context, order types.SubmitOrder) error {
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
		Quantity(order.QuantityString)

	if len(order.PriceString) > 0 {
		req.Price(order.PriceString)
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

	// avoid rate limit
	time.Sleep(100 * time.Millisecond)
	req := e.Client.NewKlinesService().
		Symbol(symbol).
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
	for _, k := range resp {
		kLines = append(kLines, types.KLine{
			Symbol:         symbol,
			Interval:       interval,
			StartTime:      time.Unix(0, k.OpenTime*int64(time.Millisecond)),
			EndTime:        time.Unix(0, k.CloseTime*int64(time.Millisecond)),
			Open:           util.MustParseFloat(k.Open),
			Close:          util.MustParseFloat(k.Close),
			High:           util.MustParseFloat(k.High),
			Low:            util.MustParseFloat(k.Low),
			Volume:         util.MustParseFloat(k.Volume),
			QuoteVolume:    util.MustParseFloat(k.QuoteAssetVolume),
			LastTradeID:    0,
			NumberOfTrades: k.TradeNum,
			Closed:         true,
		})
	}
	return kLines, nil
}

func (e *Exchange) QueryTrades(ctx context.Context, symbol string, options *types.TradeQueryOptions) (trades []types.Trade, err error) {
	req := e.Client.NewListTradesService().
		Limit(1000).
		Symbol(symbol)

	if options.Limit > 0 {
		req.Limit(int(options.Limit))
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
			if kline.EndTime.After(endTime) {
				return allKLines, nil
			}

			allKLines = append(allKLines, kline)
			startTime = kline.EndTime
		}

		// avoid rate limit
		time.Sleep(100 * time.Millisecond)
	}

	return allKLines, nil
}

func (e *Exchange) BatchQueryKLineWindows(ctx context.Context, symbol string, intervals []string, startTime, endTime time.Time) (map[string]types.KLineWindow, error) {
	batch := &types.ExchangeBatchProcessor{Exchange: e}
	klineWindows := map[string]types.KLineWindow{}
	for _, interval := range intervals {
		klines, err := batch.BatchQueryKLines(ctx, symbol, interval, startTime, endTime)
		if err != nil {
			return klineWindows, err
		}
		klineWindows[interval] = klines
	}

	return klineWindows, nil
}
