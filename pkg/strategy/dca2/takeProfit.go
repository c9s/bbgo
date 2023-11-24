package dca2

import (
	"context"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func (s *Strategy) openTakeProfitOrders(ctx context.Context) error {
	s.logger.Info("[DCA] open take profit orders")
	takeProfitOrder := s.generateTakeProfitOrder(s.Short, s.Position)
	createdOrders, err := s.OrderExecutor.SubmitOrders(ctx, takeProfitOrder)
	if err != nil {
		return err
	}

	for _, createdOrder := range createdOrders {
		s.logger.Info("SUBMIT TAKE PROFIT ORDER ", createdOrder.String())
	}

	return nil
}

func (s *Strategy) generateTakeProfitOrder(short bool, position *types.Position) types.SubmitOrder {
	takeProfitRatio := s.TakeProfitRatio
	if s.Short {
		takeProfitRatio = takeProfitRatio.Neg()
	}
	takeProfitPrice := s.Market.TruncatePrice(position.AverageCost.Mul(fixedpoint.One.Add(takeProfitRatio)))
	return types.SubmitOrder{
		Symbol:      s.Symbol,
		Market:      s.Market,
		Type:        types.OrderTypeLimit,
		Price:       takeProfitPrice,
		Side:        s.takeProfitSide,
		TimeInForce: types.TimeInForceGTC,
		Quantity:    position.GetBase().Abs(),
		Tag:         orderTag,
		GroupID:     s.OrderGroupID,
	}
}
