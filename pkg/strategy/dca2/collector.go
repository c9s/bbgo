package dca2

import (
	"context"
	"strconv"
	"time"

	"github.com/c9s/bbgo/pkg/exchange/retry"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/sirupsen/logrus"
)

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
	logger        *logrus.Entry
	symbol        string
	groupID       uint32
	filterGroupID bool

	// service
	ex                   types.Exchange
	historyService       types.ExchangeTradeHistoryService
	queryService         types.ExchangeOrderQueryService
	tradeService         types.ExchangeTradeService
	queryClosedOrderDesc descendingClosedOrderQueryService
}

func NewCollector(logger *logrus.Entry, symbol string, groupID uint32, filterGroupID bool, ex types.Exchange) *Collector {
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

	return &Collector{
		logger:               logger,
		symbol:               symbol,
		groupID:              groupID,
		filterGroupID:        filterGroupID,
		ex:                   ex,
		historyService:       historyService,
		queryService:         queryService,
		tradeService:         tradeService,
		queryClosedOrderDesc: queryClosedOrderDesc,
	}
}

func (rc Collector) CollectCurrentRound(ctx context.Context) (Round, error) {
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
		if rc.filterGroupID && order.GroupID != rc.groupID {
			continue
		}

		if order.Side == takeProfitSide && lastSide == openPositionSide {
			break
		}

		switch order.Side {
		case openPositionSide:
			currentRound.OpenPositionOrders = append(currentRound.OpenPositionOrders, order)
		case takeProfitSide:
			currentRound.TakeProfitOrders = append(currentRound.TakeProfitOrders, order)
		default:
		}

		lastSide = order.Side
	}

	return currentRound, nil
}

func (rc *Collector) CollectFinishRounds(ctx context.Context, fromOrderID uint64) ([]Round, error) {
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
		if rc.filterGroupID && order.GroupID != rc.groupID {
			continue
		}

		switch order.Side {
		case types.SideTypeBuy:
			round.OpenPositionOrders = append(round.OpenPositionOrders, order)
		case types.SideTypeSell:
			round.TakeProfitOrders = append(round.TakeProfitOrders, order)

			if order.Status != types.OrderStatusFilled {
				rc.logger.Infof("take-profit order is not filled (%s), so this round is not finished. Keep collecting", order.Status)
				continue
			}

			for _, o := range round.TakeProfitOrders {
				if types.IsActiveOrder(o) {
					// Should not happen ! but we only log it
					rc.logger.Errorf("unexpected error, there is at least one take-profit order #%d is still active, please check it. %s", o.OrderID, o.String())
				}
			}

			rounds = append(rounds, round)
			round = Round{}
		default:
			rc.logger.Errorf("there is order with unsupported side")
		}
	}

	return rounds, nil
}

// CollectRoundTrades collect the trades of the orders in the given round. The trades' fee are processed (feeProcessing = false)
func (rc *Collector) CollectRoundTrades(ctx context.Context, round Round) ([]types.Trade, error) {
	debugRoundOrders(rc.logger, "collect round trades", round)

	var roundTrades []types.Trade
	var roundOrders []types.Order = round.OpenPositionOrders

	roundOrders = append(roundOrders, round.TakeProfitOrders...)

	for _, order := range roundOrders {
		if order.ExecutedQuantity.IsZero() {
			rc.logger.Info("collect trads from order but no executed quantity ", order.String())
			continue
		} else {
			rc.logger.Info("collect trades from order ", order.String())
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
