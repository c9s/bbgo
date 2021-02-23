package service

import (
	"context"
	"errors"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"

	"github.com/c9s/bbgo/pkg/exchange/batch"
	"github.com/c9s/bbgo/pkg/types"
)

var debugSync = false

func init() {
	debugSync = viper.GetBool("DEBUG_SYNC")
}

var ErrNotImplemented = errors.New("exchange does not implement ExchangeRewardService interface")

type SyncService struct {
	TradeService  *TradeService
	OrderService  *OrderService
	RewardService *RewardService
}

func (s *SyncService) SyncRewards(ctx context.Context, exchange types.Exchange) error {
	service, ok := exchange.(types.ExchangeRewardService)
	if !ok {
		return ErrNotImplemented
	}

	var rewardKeys = map[string]struct{}{}

	var startTime time.Time
	lastRecords, err := s.RewardService.QueryLast(exchange.Name(), 50)
	if err != nil {
		return err
	}
	if len(lastRecords) > 0 {
		end := len(lastRecords) - 1
		lastRecord := lastRecords[end]
		startTime = lastRecord.CreatedAt.Time()

		for _, record := range lastRecords {
			rewardKeys[record.UUID] = struct{}{}
		}
	}

	batchQuery := &batch.RewardBatchQuery{Service: service}
	rewardsC, errC := batchQuery.Query(ctx, startTime, time.Now())

	for reward := range rewardsC {
		select {

		case <-ctx.Done():
			return ctx.Err()

		case err := <-errC:
			if err != nil {
				return err
			}

		default:

		}

		if _, ok := rewardKeys[reward.UUID]; ok {
			continue
		}

		logrus.Infof("inserting reward: %s %s %s %f %s", reward.Exchange, reward.Type, reward.Currency, reward.Quantity.Float64(), reward.CreatedAt)

		if err := s.RewardService.Insert(reward); err != nil {
			return err
		}
	}

	return <-errC
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

		if debugSync {
			logrus.Infof("found last order, start from lastID = %d since %s", lastID, startTime)
		}
	}

	b := &batch.ClosedOrderBatchQuery{Exchange: exchange}
	ordersC, errC := b.Query(ctx, symbol, startTime, time.Now(), lastID)
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

func (s *SyncService) SyncTrades(ctx context.Context, exchange types.Exchange, symbol string) error {
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
	var lastTradeID int64 = 1
	if len(lastTrades) > 0 {
		for _, t := range lastTrades {
			tradeKeys[t.Key()] = struct{}{}
		}

		lastTrade := lastTrades[len(lastTrades)-1]
		lastTradeID = lastTrade.ID
		if debugSync {
			logrus.Infof("found last trade, start from lastID = %d", lastTrade.ID)
		}
	}

	b := &batch.TradeBatchQuery{Exchange: exchange}
	tradeC, errC := b.Query(ctx, symbol, &types.TradeQueryOptions{
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

		logrus.Infof("inserting trade: %s %d %s %-4s price: %-13f volume: %-11f %5s %s",
			trade.Exchange,
			trade.ID,
			trade.Symbol,
			trade.Side,
			trade.Price,
			trade.Quantity,
			trade.MakerOrTakerLabel(),
			trade.Time.String())

		if err := s.TradeService.Insert(trade); err != nil {
			return err
		}
	}

	return <-errC
}

// SyncSessionSymbols syncs the trades from the given exchange session
func (s *SyncService) SyncSessionSymbols(ctx context.Context, exchange types.Exchange, startTime time.Time, symbols ...string) error {
	for _, symbol := range symbols {
		if err := s.SyncTrades(ctx, exchange, symbol); err != nil {
			return err
		}

		if err := s.SyncOrders(ctx, exchange, symbol, startTime); err != nil {
			return err
		}

		if err := s.SyncRewards(ctx, exchange); err != nil {
			if err == ErrNotImplemented {
				continue
			}

			return err
		}
	}

	return nil
}
