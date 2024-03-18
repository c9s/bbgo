package dca2

import (
	"context"
	"strconv"
	"time"

	"github.com/c9s/bbgo/pkg/exchange"
	maxapi "github.com/c9s/bbgo/pkg/exchange/max/maxapi"
	"github.com/c9s/bbgo/pkg/exchange/retry"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/sirupsen/logrus"
)

type RoundCollector struct {
	logger  *logrus.Entry
	symbol  string
	groupID uint32
	isMax   bool

	// service
	historyService types.ExchangeTradeHistoryService
	queryService   types.ExchangeOrderQueryService
}

func NewRoundCollector(logger *logrus.Entry, symbol string, groupID uint32, ex types.Exchange) *RoundCollector {
	isMax := exchange.IsMaxExchange(ex)
	historyService, ok := ex.(types.ExchangeTradeHistoryService)
	if !ok {
		logger.Errorf("exchange %s doesn't support ExchangeTradeHistoryService", ex.Name())
		return nil
	}

	queryService, ok := ex.(types.ExchangeOrderQueryService)
	if !ok {
		logger.Errorf("exchange %s doesn't support ExchangeOrderQueryService", ex.Name())
		return nil
	}

	return &RoundCollector{
		logger:         logger,
		symbol:         symbol,
		groupID:        groupID,
		isMax:          isMax,
		historyService: historyService,
		queryService:   queryService,
	}
}

func (rc *RoundCollector) CollectFinishRounds(ctx context.Context, fromOrderID uint64) ([]Round, error) {
	// TODO: pagination for it
	// query the orders
	rc.logger.Infof("query %s closed orders from order id #%d", rc.symbol, fromOrderID)
	orders, err := retry.QueryClosedOrdersUntilSuccessfulLite(ctx, rc.historyService, rc.symbol, time.Time{}, time.Time{}, fromOrderID)
	if err != nil {
		return nil, err
	}
	rc.logger.Infof("there are %d closed orders from order id #%d", len(orders), fromOrderID)

	var rounds []Round
	var round Round
	for _, order := range orders {
		// skip not this strategy order
		if order.GroupID != rc.groupID {
			continue
		}

		switch order.Side {
		case types.SideTypeBuy:
			round.OpenPositionOrders = append(round.OpenPositionOrders, order)
		case types.SideTypeSell:
			if !rc.isMax {
				if order.Status != types.OrderStatusFilled {
					rc.logger.Infof("take-profit order is %s not filled, so this round is not finished. Skip it", order.Status)
					continue
				}
			} else {
				if !maxapi.IsFilledOrderState(maxapi.OrderState(order.OriginalStatus)) {
					rc.logger.Infof("isMax and take-profit order is %s not done or finalizing, so this round is not finished. Skip it", order.OriginalStatus)
					continue
				}
			}

			round.TakeProfitOrder = order
			rounds = append(rounds, round)
			round = Round{}
		default:
			rc.logger.Errorf("there is order with unsupported side")
		}
	}

	return rounds, nil
}

func (rc *RoundCollector) CollectRoundTrades(ctx context.Context, round Round) ([]types.Trade, error) {
	debugRoundOrders(rc.logger, "collect round trades", round)

	var roundTrades []types.Trade

	var roundOrders []types.Order = round.OpenPositionOrders
	roundOrders = append(roundOrders, round.TakeProfitOrder)

	for _, order := range roundOrders {
		rc.logger.Infof("collect trades from order: %s", order.String())
		if order.ExecutedQuantity.Sign() == 0 {
			rc.logger.Info("collect trads from order but no executed quantity ", order.String())
			continue
		} else {
			rc.logger.Info("collect trades from order ", order.String())
		}

		trades, err := retry.QueryOrderTradesUntilSuccessfulLite(ctx, rc.queryService, types.OrderQuery{
			Symbol:  order.Symbol,
			OrderID: strconv.FormatUint(order.OrderID, 10),
		})

		if err != nil {
			return nil, err
		}

		roundTrades = append(roundTrades, trades...)
	}

	return roundTrades, nil
}
