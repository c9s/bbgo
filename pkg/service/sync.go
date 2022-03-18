package service

import (
	"context"
	"errors"
	"time"

	"github.com/c9s/bbgo/pkg/cache"

	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
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

func paperTrade() bool {
	v, ok := util.GetEnvVarBool("PAPER_TRADE")
	return ok && v
}

// SyncSessionSymbols syncs the trades from the given exchange session
func (s *SyncService) SyncSessionSymbols(ctx context.Context, exchange types.Exchange, startTime time.Time, symbols ...string) error {
	markets, err := cache.LoadExchangeMarketsWithCache(ctx, exchange)
	if err != nil {
		return err
	}

	for _, symbol := range symbols {
		if _, ok := markets[symbol]; ok {
			log.Infof("syncing %s %s trades...", exchange.Name(), symbol)
			if err := s.TradeService.Sync(ctx, exchange, symbol, startTime); err != nil {
				return err
			}

			log.Infof("syncing %s %s orders...", exchange.Name(), symbol)
			if err := s.OrderService.Sync(ctx, exchange, symbol, startTime); err != nil {
				return err
			}
		}
	}

	if paperTrade() {
		return nil
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
