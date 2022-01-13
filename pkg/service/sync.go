package service

import (
	"context"
	"errors"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/types"
)

var ErrNotImplemented = errors.New("not implemented")
var ErrExchangeRewardServiceNotImplemented = errors.New("exchange does not implement ExchangeRewardService interface")

type SyncService struct {
	TradeService    *TradeService
	OrderService    *OrderService
	RewardService   *RewardService
	WithdrawService *WithdrawService
	DepositService  *DepositService
}

// SyncSessionSymbols syncs the trades from the given exchange session
func (s *SyncService) SyncSessionSymbols(ctx context.Context, exchange types.Exchange, startTime time.Time, symbols ...string) error {
	for _, symbol := range symbols {
		log.Infof("syncing %s %s trades...", exchange.Name(), symbol)
		// TODO: bbgo import cycle error
		//markets, err := bbgo.LoadExchangeMarketsWithCache(ctx, exchange)
		markets, err := exchange.QueryMarkets(ctx)
		if err != nil {
			return err
		}

		if _, ok := markets[symbol]; ok {
			if err := s.TradeService.Sync(ctx, exchange, symbol, startTime); err != nil {
				return err
			}
			log.Infof("syncing %s %s orders...", exchange.Name(), symbol)
			if err := s.OrderService.Sync(ctx, exchange, symbol, startTime); err != nil {
				return err
			}
		}
	}

	log.Infof("syncing %s deposit records...", exchange.Name())
	if err := s.DepositService.Sync(ctx, exchange); err != nil {
		if err != ErrNotImplemented {
			return err
		}
	}

	log.Infof("syncing %s withdraw records...", exchange.Name())
	if err := s.WithdrawService.Sync(ctx, exchange); err != nil {
		if err != ErrNotImplemented {
			return err
		}
	}

	log.Infof("syncing %s reward records...", exchange.Name())
	if err := s.RewardService.Sync(ctx, exchange); err != nil {
		if err != ErrExchangeRewardServiceNotImplemented {
			log.Infof("%s reward service is not supported", exchange.Name())
			return err
		}
	}

	return nil
}
