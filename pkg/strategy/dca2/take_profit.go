package dca2

import (
	"context"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func (s *Strategy) placeTakeProfitOrders(ctx context.Context) error {
	s.logger.Info("[DCA] start placing take profit orders")
	order := generateTakeProfitOrder(s.Market, s.TakeProfitRatio, s.Position, s.OrderGroupID)
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
