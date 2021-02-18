package service

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"

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
