package service

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	batch2 "github.com/c9s/bbgo/pkg/exchange/batch"
	"github.com/c9s/bbgo/pkg/types"
)

type SyncService struct {
	TradeService *TradeService
	OrderService *OrderService
}

func (s *SyncService) SyncOrders(ctx context.Context, exchange types.Exchange, symbol string, startTime time.Time) error {
	isMargin := false
	isIsolated := false
	if marginExchange, ok := exchange.(types.MarginExchange); ok {
		marginSettings := marginExchange.GetMarginSettings()
		isMargin = marginSettings.IsMargin
		isIsolated = marginSettings.IsIsolatedMargin
		if marginSettings.IsIsolatedMargin {
			symbol = marginSettings.IsolatedMarginSymbol
		}
	}

	lastOrder, err := s.OrderService.QueryLast(exchange.Name(), symbol, isMargin, isIsolated)
	if err != nil {
		return err
	}

	var lastID uint64 = 0
	if lastOrder != nil {
		lastID = lastOrder.OrderID
		startTime = lastOrder.CreationTime.Time()

		logrus.Infof("found last order, start from lastID = %d since %s", lastID, startTime)
	}

	batch := &batch2.ExchangeBatchProcessor{Exchange: exchange}
	ordersC, errC := batch.BatchQueryClosedOrders(ctx, symbol, startTime, time.Now(), lastID)
	for order := range ordersC {
		select {

		case <-ctx.Done():
			return ctx.Err()

		case err := <-errC:
			if err != nil {
				return err
			}

		default:

		}

		if err := s.OrderService.Insert(order); err != nil {
			return err
		}
	}

	return <-errC
}

func (s *SyncService) SyncTrades(ctx context.Context, exchange types.Exchange, symbol string, startTime time.Time) error {
	isMargin := false
	isIsolated := false
	if marginExchange, ok := exchange.(types.MarginExchange); ok {
		marginSettings := marginExchange.GetMarginSettings()
		isMargin = marginSettings.IsMargin
		isIsolated = marginSettings.IsIsolatedMargin
		if marginSettings.IsIsolatedMargin {
			symbol = marginSettings.IsolatedMarginSymbol
		}
	}

	lastTrades, err := s.TradeService.QueryLast(exchange.Name(), symbol, isMargin, isIsolated, 10)
	if err != nil {
		return err
	}

	var tradeKeys = map[types.TradeKey]struct{}{}
	var lastTradeID int64 = 0
	if len(lastTrades) > 0 {
		for _, t := range lastTrades {
			tradeKeys[t.Key()] = struct{}{}
		}

		lastTrade := lastTrades[len(lastTrades)-1]
		lastTradeID = lastTrade.ID

		startTime = time.Time(lastTrade.Time)
		logrus.Debugf("found last trade, start from lastID = %d since %s", lastTrade.ID, startTime)
	}

	batch := &batch2.ExchangeBatchProcessor{Exchange: exchange}
	tradeC, errC := batch.BatchQueryTrades(ctx, symbol, &types.TradeQueryOptions{
		StartTime:   &startTime,
		LastTradeID: lastTradeID,
	})

	for trade := range tradeC {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case err := <-errC:
			if err != nil {
				return err
			}

		default:
		}

		key := trade.Key()
		if _, ok := tradeKeys[key]; ok {
			continue
		}

		tradeKeys[key] = struct{}{}

		logrus.Infof("inserting trade: %d %s %-4s price: %-13f volume: %-11f %5s %s", trade.ID, trade.Symbol, trade.Side, trade.Price, trade.Quantity, trade.MakerOrTakerLabel(), trade.Time.String())

		if err := s.TradeService.Insert(trade); err != nil {
			return err
		}
	}

	return <-errC
}


// SyncSessionSymbols syncs the trades from the given exchange session
func (s *SyncService) SyncSessionSymbols(ctx context.Context, session *bbgo.ExchangeSession, startTime time.Time, symbols ...string) error {
	for _, symbol := range symbols {
		logrus.Debugf("syncing trades from exchange session %s...", session.Name)
		if err := s.SyncTrades(ctx, session.Exchange, symbol, startTime); err != nil {
			return err
		}

		logrus.Debugf("syncing orders from exchange session %s...", session.Name)
		if err := s.SyncOrders(ctx, session.Exchange, symbol, startTime); err != nil {
			return err
		}
	}

	return nil
}

func (s *SyncService) SyncSession(ctx context.Context, session *bbgo.ExchangeSession, startTime time.Time, defaultSymbols ...string) error {
	symbols, err := getSessionSymbols(session, defaultSymbols...)
	if err != nil {
		return err
	}

	return s.SyncSessionSymbols(ctx, session, startTime, symbols...)
}

func getSessionSymbols(session *bbgo.ExchangeSession, defaultSymbols ...string) ([]string, error) {
	if session.IsolatedMargin {
		return []string{session.IsolatedMarginSymbol}, nil
	}

	if len(defaultSymbols) > 0 {
		return defaultSymbols, nil
	}

	return FindPossibleSymbols(session)
}


func FindPossibleSymbols(session *bbgo.ExchangeSession) (symbols []string, err error) {
	var balances = session.Account.Balances()
	var fiatCurrencies = []string{"USDC", "USDT", "USD", "TWD", "EUR", "GBP"}
	var fiatAssets []string

	for _, currency := range fiatCurrencies {
		if balance, ok := balances[currency]; ok && balance.Total() > 0 {
			fiatAssets = append(fiatAssets, currency)
		}
	}

	var symbolMap = map[string]struct{}{}

	for _, market := range session.Markets() {
		// ignore the markets that are not fiat currency markets
		if !stringSliceContains(fiatAssets, market.QuoteCurrency) {
			continue
		}

		// ignore the asset that we don't have in the balance sheet
		balance, hasAsset := balances[market.BaseCurrency]
		if !hasAsset || balance.Total() == 0 {
			continue
		}

		symbolMap[market.Symbol] = struct{}{}
	}

	for s := range symbolMap {
		symbols = append(symbols, s)
	}

	return symbols, nil
}


func stringSliceContains(slice []string, needle string) bool {
	for _, s := range slice {
		if s == needle {
			return true
		}
	}

	return false
}
