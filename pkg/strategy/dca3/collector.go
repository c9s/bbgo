package dca3

import (
	"context"
	"strconv"
	"time"

	"github.com/c9s/bbgo/pkg/exchange/retry"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/sirupsen/logrus"
)

type CollectorQueryService interface {
	types.ExchangeTradeHistoryService
	types.ExchangeOrderQueryService
	QueryClosedOrdersDesc(ctx context.Context, symbol string, since, until time.Time, lastOrderID uint64) ([]types.Order, error)
}

// Round contains the open-position orders and the take-profit orders
// 1. len(OpenPositionOrders) == 0 -> not open position
// 2. len(TakeProfitOrders) == 0   -> not in the take-profit stage
// 3. There are take-profit orders only when open-position orders are cancelled
// 4. We need to make sure the order: open-position (BUY) -> take-profit (SELL) -> open-position (BUY) -> take-profit (SELL) -> ...
// 5. When there is one filled take-profit order, this round must be finished. We need to verify all take-profit orders are not active
type Round struct {
	OpenPositionOrders []types.Order
	TakeProfitOrders   []types.Order
}

type Collector struct {
	logger        logrus.FieldLogger
	symbol        string
	groupID       uint32
	filterGroupID bool

	// service
	ex           types.Exchange
	queryService CollectorQueryService
}

func NewCollector(logger logrus.FieldLogger, symbol string, groupID uint32, filterGroupID bool, ex types.Exchange, queryService CollectorQueryService) *Collector {
	return &Collector{
		logger:        logger,
		symbol:        symbol,
		groupID:       groupID,
		filterGroupID: filterGroupID,
		ex:            ex,
		queryService:  queryService,
	}
}

func (rc Collector) isValidOrder(order types.Order) bool {
	if rc.filterGroupID && order.GroupID != rc.groupID {
		// skip the order not belong to this group
		return false
	}

	if order.Status == types.OrderStatusRejected {
		// skip the rejected order
		return false
	}

	if order.Status == types.OrderStatusExpired && order.ExecutedQuantity.IsZero() {
		// skip the expired order without executed quantity
		return false
	}

	if order.Status == types.OrderStatusCanceled && order.ExecutedQuantity.IsZero() {
		// skip the cancelled order
		return false
	}

	return true
}

func (rc Collector) CollectCurrentRound(ctx context.Context, sinceLimit time.Time) (Round, error) {
	openOrders, err := retry.QueryOpenOrdersUntilSuccessful(ctx, rc.ex, rc.symbol)
	if err != nil {
		return Round{}, err
	}

	var closedOrders []types.Order
	var op = func() (err2 error) {
		closedOrders, err2 = rc.queryService.QueryClosedOrdersDesc(ctx, rc.symbol, sinceLimit, time.Now(), 0)
		return err2
	}
	if err := retry.GeneralBackoff(ctx, op); err != nil {
		return Round{}, err
	}

	var allOrders []types.Order
	allOrders = append(allOrders, openOrders...)
	allOrders = append(allOrders, closedOrders...)

	types.SortOrdersDescending(allOrders)

	var currentRound Round
	lastSide := TakeProfitSide
	for _, order := range allOrders {
		if !rc.isValidOrder(order) {
			continue
		}

		if order.Side == TakeProfitSide && lastSide == OpenPositionSide {
			break
		}

		switch order.Side {
		case OpenPositionSide:
			currentRound.OpenPositionOrders = append(currentRound.OpenPositionOrders, order)
		case TakeProfitSide:
			currentRound.TakeProfitOrders = append(currentRound.TakeProfitOrders, order)
		default:
		}

		lastSide = order.Side
	}

	debugRoundOrders(rc.logger, "CURRENT", currentRound)
	return currentRound, nil
}

func (rc *Collector) CollectRoundsFromOrderID(ctx context.Context, fromOrderID uint64) ([]Round, error) {
	// TODO: pagination for it
	// query the orders
	rc.logger.Infof("query %s closed orders from order id #%d", rc.symbol, fromOrderID)
	orders, err := retry.QueryClosedOrdersUntilSuccessfulLite(ctx, rc.queryService, rc.symbol, time.Time{}, time.Time{}, fromOrderID)
	if err != nil {
		return nil, err
	}
	rc.logger.Infof("there are %d closed orders from order id #%d", len(orders), fromOrderID)
	debugOrders(rc.logger, "CLOSED", orders)

	if len(orders) == 0 {
		return nil, nil
	}

	var lastSide types.SideType = OpenPositionSide
	var round Round
	var rounds []Round
	for _, order := range orders {
		if !rc.isValidOrder(order) {
			continue
		}

		switch order.Side {
		case OpenPositionSide:
			if lastSide == TakeProfitSide {
				rounds = append(rounds, round)
				round = Round{}
			}
			round.OpenPositionOrders = append(round.OpenPositionOrders, order)
		case TakeProfitSide:
			round.TakeProfitOrders = append(round.TakeProfitOrders, order)
		default:
			rc.logger.Errorf("there is order with unsupported side")
		}

		lastSide = order.Side
	}
	rounds = append(rounds, round)

	rc.logger.Infof("there are %d finished rounds from order id #%d", len(rounds), fromOrderID)
	return rounds, nil
}

func isFinishedRound(round Round, position *types.Position, minQuantity fixedpoint.Value) bool {
	// if there is no take-profit orders, it means this round is not finished
	if len(round.TakeProfitOrders) == 0 {
		return false
	}

	// if there is still active orders, it means this round is not finished
	for _, order := range round.OpenPositionOrders {
		if types.IsActiveOrder(order) {
			return false
		}
	}

	for _, order := range round.TakeProfitOrders {
		if types.IsActiveOrder(order) {
			return false
		}
	}

	// we need to make sure the position base is less than MOQ
	return position.GetBase().Compare(minQuantity) < 0
}

// CollectRoundTrades collect the trades of the orders in the given round. The trades' fee are processed (feeProcessing = false)
func (rc *Collector) CollectRoundTrades(ctx context.Context, round Round) ([]types.Trade, error) {
	debugRoundOrders(rc.logger, "TRADE", round)

	var roundTrades []types.Trade
	var roundOrders []types.Order = round.OpenPositionOrders

	roundOrders = append(roundOrders, round.TakeProfitOrders...)

	for i, order := range roundOrders {
		if order.ExecutedQuantity.IsZero() {
			rc.logger.Infof("collect trades from #%d order but no executed quantity %s", i, order.String())
			continue
		} else {
			rc.logger.Infof("collect trades from #%d order %s", i, order.String())
		}

		// QueryOrderTradesUntilSuccessful will query trades and their feeProcessing = false
		trades, err := retry.QueryOrderTradesUntilSuccessful(ctx, rc.queryService, types.OrderQuery{
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
