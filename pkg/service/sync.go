package service

import (
	"context"
	"errors"
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/time/rate"

	"github.com/c9s/bbgo/pkg/exchange/batch"
	"github.com/c9s/bbgo/pkg/types"
)

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


	var startTime time.Time
	lastRecords, err := s.RewardService.QueryLast(exchange.Name(), 10)
	if err != nil {
		return err
	}
	if len(lastRecords) > 0 {
		end :=  len(lastRecords) - 1
		lastRecord := lastRecords[end]
		startTime = lastRecord.CreatedAt.Time()
	}

	batchQuery := &RewardBatchQuery{Service: service}
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

		if err := s.RewardService.Insert(reward); err != nil {
			return err
		}
	}

	return <-errC
}

type RewardBatchQuery struct {
	Service types.ExchangeRewardService
}

func (q *RewardBatchQuery) Query(ctx context.Context, startTime, endTime time.Time) (c chan types.Reward, errC chan error) {
	c = make(chan types.Reward, 500)
	errC = make(chan error, 1)

	go func() {
		limiter := rate.NewLimiter(rate.Every(5*time.Second), 2) // from binance (original 1200, use 1000 for safety)

		defer close(c)
		defer close(errC)

		rewardKeys := make(map[string]struct{}, 500)

		for startTime.Before(endTime) {
			if err := limiter.Wait(ctx); err != nil {
				logrus.WithError(err).Error("rate limit error")
			}

			logrus.Infof("batch querying rewards %s <=> %s", startTime, endTime)

			rewards, err := q.Service.QueryRewards(ctx, startTime)
			if err != nil {
				errC <- err
				return
			}

			if len(rewards) == 0 {
				return
			}

			for _, o := range rewards {
				if _, ok := rewardKeys[o.UUID]; ok {
					logrus.Infof("skipping duplicated order id: %s", o.UUID)
					continue
				}

				if o.CreatedAt.Time().After(endTime) {
					// stop batch query
					return
				}

				c <- o
				startTime = o.CreatedAt.Time()
				rewardKeys[o.UUID] = struct{}{}
			}
		}

	}()

	return c, errC
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

	b := &batch.ExchangeBatchProcessor{Exchange: exchange}
	ordersC, errC := b.BatchQueryClosedOrders(ctx, symbol, startTime, time.Now(), lastID)
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
		logrus.Debugf("found last trade, start from lastID = %d", lastTrade.ID)
	}

	b := &batch.ExchangeBatchProcessor{Exchange: exchange}
	tradeC, errC := b.BatchQueryTrades(ctx, symbol, &types.TradeQueryOptions{
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


		if err := s.SyncRewards(ctx, exchange) ; err != nil {
			if err == ErrNotImplemented {
				continue
			}

			return err
		}
	}

	return nil
}
