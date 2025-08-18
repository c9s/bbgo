package tradingutil

import (
	"context"
	"fmt"

	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/exchange/retry"
	"github.com/c9s/bbgo/pkg/types"
)

type CancelAllOrdersService interface {
	CancelAllOrders(ctx context.Context) ([]types.Order, error)
}

type CancelAllOrdersBySymbolService interface {
	CancelOrdersBySymbol(ctx context.Context, symbol string) ([]types.Order, error)
}

type CancelAllOrdersByGroupIDService interface {
	CancelOrdersByGroupID(ctx context.Context, groupID uint32) ([]types.Order, error)
}

// UniversalCancelAllOrders checks if the exchange instance supports the best order cancel strategy
// it tries the first interface CancelAllOrdersService that does not need any existing order information or symbol information.
//
// if CancelAllOrdersService is not supported, then it tries CancelAllOrdersBySymbolService which needs at least one symbol
// for the cancel api request.
func UniversalCancelAllOrders(ctx context.Context, exchange types.Exchange, symbol string, openOrders []types.Order) error {
	if service, ok := exchange.(CancelAllOrdersService); ok {
		if _, err := service.CancelAllOrders(ctx); err == nil {
			return nil
		} else {
			log.WithError(err).Errorf("unable to cancel all orders")
		}
	}

	var anyErr error
	if service, ok := exchange.(CancelAllOrdersBySymbolService); ok {
		if len(symbol) > 0 {
			_, anyErr = service.CancelOrdersBySymbol(ctx, symbol)
		} else if len(openOrders) > 0 {
			var orderSymbols = CollectOrderSymbols(openOrders)
			for _, orderSymbol := range orderSymbols {
				_, err := service.CancelOrdersBySymbol(ctx, orderSymbol)
				if err != nil {
					anyErr = err
				}
			}
		}

		if anyErr == nil {
			return nil
		}
	}

	if len(openOrders) > 0 {
		if service, ok := exchange.(CancelAllOrdersByGroupIDService); ok {
			var groupIds = CollectOrderGroupIds(openOrders)
			for _, groupId := range groupIds {
				if _, err := service.CancelOrdersByGroupID(ctx, groupId); err != nil {
					anyErr = err
				}
			}

			if anyErr == nil {
				return nil
			}
		}

		if anyErr != nil {
			return anyErr
		}
	}

	// if we have no open order, then use the exchange service query to get the open orders and then cancel them all
	if len(openOrders) == 0 {
		if len(symbol) == 0 {
			log.Warnf("empty open orders, unable to call specific cancel all orders api, skip")
			return nil
		}

		var err error
		openOrders, err = retry.QueryOpenOrdersUntilSuccessful(ctx, exchange, symbol)
		if err != nil {
			return err
		}

		return retry.CancelOrdersUntilSuccessful(ctx, exchange, openOrders...)
	}

	return fmt.Errorf("unable to cancel all orders, openOrders:%+v", openOrders)
}

func CollectOrderGroupIds(orders []types.Order) (groupIds []uint32) {
	groupIdMap := map[uint32]struct{}{}
	for _, o := range orders {
		if o.GroupID > 0 {
			groupIdMap[o.GroupID] = struct{}{}
		}
	}

	for id := range groupIdMap {
		groupIds = append(groupIds, id)
	}

	return groupIds
}

func CollectOrderSymbols(orders []types.Order) (symbols []string) {
	symbolMap := map[string]struct{}{}
	for _, o := range orders {
		symbolMap[o.Symbol] = struct{}{}
	}

	for s := range symbolMap {
		symbols = append(symbols, s)
	}

	return symbols
}

func CollectOpenOrders(ctx context.Context, ex types.Exchange, symbols ...string) ([]types.Order, error) {
	var collectedOrders []types.Order
	for _, symbol := range symbols {
		openOrders, err := retry.QueryOpenOrdersUntilSuccessful(ctx, ex, symbol)
		if err != nil {
			return nil, err
		}

		collectedOrders = append(collectedOrders, openOrders...)
	}

	return collectedOrders, nil
}
