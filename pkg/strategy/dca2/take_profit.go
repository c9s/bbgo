package dca2

import (
	"context"
	"fmt"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/pkg/errors"
)

func (s *Strategy) placeTakeProfitOrders(ctx context.Context) error {
	s.logger.Info("start placing take profit orders")
	currentRound, err := s.roundCollector.CollectCurrentRound(ctx)
	if currentRound.TakeProfitOrder.OrderID != 0 {
		return fmt.Errorf("there is a take-profit order before placing the take-profit order, please check it")
	}

	trades, err := s.roundCollector.CollectRoundTrades(ctx, currentRound)
	if err != nil {
		return errors.Wrap(err, "failed to place the take-profit order when collecting round trades")
	}

	position := types.NewPositionFromMarket(s.Market)

	for _, trade := range trades {
		if trade.FeeProcessing {
			return fmt.Errorf("failed to place the take-profit order because there is a trade's fee not ready")
		}

		position.AddTrade(trade)
	}

	s.logger.Infof("placeTakeProfitOrders: %s", position.String())

	order := generateTakeProfitOrder(s.Market, s.TakeProfitRatio, position, s.OrderGroupID)
	createdOrders, err := s.OrderExecutor.SubmitOrders(ctx, order)
	if err != nil {
		return err
	}

	for _, createdOrder := range createdOrders {
		s.logger.Info("SUBMIT TAKE PROFIT ORDER ", createdOrder.String())
	}

	return nil
}

func generateTakeProfitOrder(market types.Market, takeProfitRatio fixedpoint.Value, position *types.Position, orderGroupID uint32) types.SubmitOrder {
	side := types.SideTypeSell
	takeProfitPrice := market.TruncatePrice(position.AverageCost.Mul(fixedpoint.One.Add(takeProfitRatio)))
	return types.SubmitOrder{
		Symbol:      market.Symbol,
		Market:      market,
		Type:        types.OrderTypeLimit,
		Price:       takeProfitPrice,
		Side:        side,
		TimeInForce: types.TimeInForceGTC,
		Quantity:    position.GetBase().Abs(),
		Tag:         orderTag,
		GroupID:     orderGroupID,
	}
}
