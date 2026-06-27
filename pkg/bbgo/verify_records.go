package bbgo

import (
	"context"
	"fmt"
	"time"

	"github.com/c9s/bbgo/pkg/exchange/batch"
	"github.com/c9s/bbgo/pkg/service"
	"github.com/c9s/bbgo/pkg/types"
	log "github.com/sirupsen/logrus"
)

func verifyRecords(ctx context.Context, sessions map[string]*ExchangeSession, orderService *service.OrderService, tradeService *service.TradeService, userConfig *Config) error {
	// nil checks
	if userConfig == nil || userConfig.Sync == nil {
		return nil
	}

	// check enabled
	verifyCfg := userConfig.Sync.Verify
	if verifyCfg == nil || !verifyCfg.Enabled {
		return nil
	}

	// default config
	if verifyCfg.Action == "" {
		verifyCfg.Action = VerifyActionWarn
	}

	if tradeService == nil {
		log.Warn("trade service is not configured, skipping verification")
		return nil
	}

	if orderService == nil {
		log.Warn("order service is not configured, skipping verificaiton")
		return nil
	}

	period := verifyCfg.Period.Duration()
	if period <= 0 {
		log.Warn("verify period is zero or negative, skipping verification")
		return nil
	}

	var since, until time.Time
	if verifyCfg.Since != nil {
		since = verifyCfg.Since.Time()
		until = since.Add(period)
	} else {
		until = time.Now()
		since = until.Add(-period)
	}

	log.Infof("verifying records from %s to %s", since.Format(time.RFC3339), until.Format(time.RFC3339))

	var verifySessions []*ExchangeSession
	if len(verifyCfg.Sessions) > 0 {
		for _, name := range verifyCfg.Sessions {
			if session, ok := sessions[name]; ok {
				verifySessions = append(verifySessions, session)
			}
		}
	}

	sessionSymbolsMap, restSymbols := categorizeSyncSymbol(userConfig.Sync.Symbols)

	for _, session := range verifySessions {
		if session.IsInMaintenance() {
			continue
		}

		api, ok := session.Exchange.(types.ExchangeTradeHistoryService)
		if !ok {
			log.Warnf("session %s exchange does not implement ExchangeTradeHistoryService, skipping verification", session.Name)
			continue
		}

		var symbols []string
		if ss, ok := sessionSymbolsMap[session.Name]; ok {
			symbols = ss
		} else if len(restSymbols) > 0 {
			symbols = restSymbols
		} else {
			var err error
			symbols, err = session.getSessionSymbols()
			if err != nil {
				log.WithError(err).Warnf("session %s: failed to get symbols for verification", session.Name)
				continue
			}
		}

		logger := log.WithFields(log.Fields{
			"session":   session.Name,
			"component": "syncVerify",
		})
		for _, symbol := range symbols {
			// verify orders
			orphanOrders, err := verifyOrdersForSymbol(ctx, orderService, api, session.Exchange.Name(), symbol, since, until)
			if err != nil {
				logger.WithError(err).Warnf("order verification query failed: %s", symbol)
			} else if len(orphanOrders) > 0 {
				logger.Infof("found %d orphan order records for %s", len(orphanOrders), symbol)

				switch verifyCfg.Action {
				case VerifyActionWarn:
					for _, o := range orphanOrders {
						logger.Warnf("orphan order detected: %s", o)
					}
				case VerifyActionDelete:
					gids := make([]uint64, 0, len(orphanOrders))
					for _, o := range orphanOrders {
						gids = append(gids, o.GID)
						logger.Warnf("deleting orphan order: %s", o)
					}
					if err := orderService.DeleteByGID(ctx, gids); err != nil {
						logger.WithError(err).Errorf("failed to delete orphan orders for %s", symbol)
					}
				default:
					logger.Warnf("unknown verify action for orders: %s", verifyCfg.Action)
				}
			} else {
				logger.Infof("no orphan orders found: %s", symbol)
			}

			// verify trades
			orphanTrades, err := verifyTradesForSymbol(ctx, tradeService, api, session.Exchange.Name(), symbol, since, until)
			if err != nil {
				logger.WithError(err).Warnf("trade verification query failed: %s", symbol)
			} else if len(orphanTrades) > 0 {
				logger.Infof("found %d orphan trade records for %s", len(orphanTrades), symbol)

				switch verifyCfg.Action {
				case VerifyActionWarn:
					for _, t := range orphanTrades {
						logger.Warnf("orphan trade detected: %s", t)
					}

				case VerifyActionDelete:
					gids := make([]int64, 0, len(orphanTrades))
					for _, t := range orphanTrades {
						gids = append(gids, t.GID)
						logger.Infof("deleting orphan trade: %s", t)
					}
					if err := tradeService.DeleteByGID(ctx, gids); err != nil {
						logger.WithError(err).Errorf("session %s symbol %s: failed to delete orphan trades", session.Name, symbol)
					}
				default:
					logger.Warnf("unknown verify action for trades: %s", verifyCfg.Action)
				}
			} else {
				logger.Infof("no orphan trades found: %s", symbol)
			}
		}
	}

	return nil
}

func verifyOrdersForSymbol(
	ctx context.Context,
	orderService *service.OrderService,
	api types.ExchangeTradeHistoryService,
	exchangeName types.ExchangeName,
	symbol string,
	since, until time.Time,
) ([]types.Order, error) {
	aggOrders, err := orderService.Query(service.QueryOrdersOptions{
		Exchange: exchangeName,
		Symbol:   symbol,
		Since:    &since,
		Until:    &until,
		Ordering: "ASC",
	})
	if err != nil {
		return nil, fmt.Errorf("query local orders: %w", err)
	}

	if len(aggOrders) == 0 {
		return nil, nil
	}

	localOrders := make([]types.Order, 0, len(aggOrders))
	for _, ao := range aggOrders {
		localOrders = append(localOrders, ao.Order)
	}

	bq := &batch.ClosedOrderBatchQuery{ExchangeTradeHistoryService: api}
	orderChan, errChan := bq.Query(ctx, symbol, since, until, 0)

	exchangeOrderIDs := make(map[uint64]struct{})
	for order := range orderChan {
		exchangeOrderIDs[order.OrderID] = struct{}{}
	}
	if err := <-errChan; err != nil {
		return nil, fmt.Errorf("query exchange orders: %w", err)
	}

	var orphans []types.Order
	for _, localOrder := range localOrders {
		if _, exists := exchangeOrderIDs[localOrder.OrderID]; !exists {
			orphans = append(orphans, localOrder)
		}
	}

	return orphans, nil
}

func verifyTradesForSymbol(
	ctx context.Context,
	tradeService *service.TradeService,
	api types.ExchangeTradeHistoryService,
	exchangeName types.ExchangeName,
	symbol string,
	since, until time.Time,
) ([]types.Trade, error) {
	localTrades, err := tradeService.Query(service.QueryTradesOptions{
		Exchange: exchangeName,
		Symbol:   symbol,
		Since:    &since,
		Until:    &until,
		Ordering: "ASC",
	})
	if err != nil {
		return nil, fmt.Errorf("query local trades: %w", err)
	}

	if len(localTrades) == 0 {
		return nil, nil
	}

	bq := &batch.TradeBatchQuery{ExchangeTradeHistoryService: api}
	tradeChan, errChan := bq.Query(ctx, symbol, &types.TradeQueryOptions{
		StartTime: &since,
		EndTime:   &until,
	})

	exchangeTradeIDs := make(map[uint64]struct{})
	for trade := range tradeChan {
		exchangeTradeIDs[trade.ID] = struct{}{}
	}
	if err := <-errChan; err != nil {
		return nil, fmt.Errorf("query exchange trades: %w", err)
	}

	var orphans []types.Trade
	for _, localTrade := range localTrades {
		if _, exists := exchangeTradeIDs[localTrade.ID]; !exists {
			orphans = append(orphans, localTrade)
		}
	}

	return orphans, nil
}
