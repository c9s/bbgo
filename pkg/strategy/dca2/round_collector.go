package dca2

import (
	"context"
	"fmt"
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
	ex                   types.Exchange
	historyService       types.ExchangeTradeHistoryService
	queryService         types.ExchangeOrderQueryService
	tradeService         types.ExchangeTradeService
	queryClosedOrderDesc descendingClosedOrderQueryService
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

	tradeService, ok := ex.(types.ExchangeTradeService)
	if !ok {
		logger.Errorf("exchange %s doesn't support ExchangeTradeService", ex.Name())
		return nil
	}

	queryClosedOrderDesc, ok := ex.(descendingClosedOrderQueryService)
	if !ok {
		logger.Errorf("exchange %s doesn't support query closed orders desc", ex.Name())
		return nil
	}

	return &RoundCollector{
		logger:               logger,
		symbol:               symbol,
		groupID:              groupID,
		isMax:                isMax,
		ex:                   ex,
		historyService:       historyService,
		queryService:         queryService,
		tradeService:         tradeService,
		queryClosedOrderDesc: queryClosedOrderDesc,
	}
}

func (rc RoundCollector) CollectCurrentRound(ctx context.Context) (Round, error) {
	openOrders, err := retry.QueryOpenOrdersUntilSuccessful(ctx, rc.ex, rc.symbol)
	if err != nil {
		return Round{}, err
	}

	var closedOrders []types.Order
	var op = func() (err2 error) {
		closedOrders, err2 = rc.queryClosedOrderDesc.QueryClosedOrdersDesc(ctx, rc.symbol, recoverSinceLimit, time.Now(), 0)
		return err2
	}
	if err := retry.GeneralBackoff(ctx, op); err != nil {
		return Round{}, err
	}

	openPositionSide := types.SideTypeBuy
	takeProfitSide := types.SideTypeSell

	var allOrders []types.Order
	allOrders = append(allOrders, openOrders...)
	allOrders = append(allOrders, closedOrders...)

	types.SortOrdersDescending(allOrders)

	var currentRound Round
	lastSide := takeProfitSide
	for _, order := range allOrders {
		// group id filter is used for debug when local running
		if order.GroupID != rc.groupID {
			continue
		}

		if order.Side == takeProfitSide && lastSide == openPositionSide {
			break
		}

		switch order.Side {
		case openPositionSide:
			currentRound.OpenPositionOrders = append(currentRound.OpenPositionOrders, order)
		case takeProfitSide:
			if currentRound.TakeProfitOrder.OrderID != 0 {
				return currentRound, fmt.Errorf("there are two take-profit orders in one round, please check it")
			}
			currentRound.TakeProfitOrder = order
		default:
		}

		lastSide = order.Side
	}

	return currentRound, nil
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

	// if the take-profit order's OrderID == 0 -> no take-profit order.
	if round.TakeProfitOrder.OrderID != 0 {
		roundOrders = append(roundOrders, round.TakeProfitOrder)
	}

	for _, order := range roundOrders {
		rc.logger.Infof("collect trades from order: %s", order.String())
		if order.ExecutedQuantity.IsZero() {
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
