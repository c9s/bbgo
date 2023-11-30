package dca2

import (
	"context"
	"fmt"
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func (s *Strategy) placeMakerOrders(ctx context.Context) error {
	s.logger.Infof("[DCA] start placing maker orders")
	price, err := s.getBestPriceUntilSuccess(ctx, s.Short)
	if err != nil {
		return err
	}

	orders, err := s.generateMakerOrder(s.Short, s.Budget, price, s.PriceDeviation, s.MaxOrderNum)
	if err != nil {
		return err
	}

	createdOrders, err := s.OrderExecutor.SubmitOrders(ctx, orders...)
	if err != nil {
		return err
	}

	s.debugOrders(createdOrders)

	return nil
}

func (s *Strategy) getBestPriceUntilSuccess(ctx context.Context, short bool) (fixedpoint.Value, error) {
	var err error
	var ticker *types.Ticker
	for try := 1; try <= 100; try++ {
		ticker, err = s.Session.Exchange.QueryTicker(ctx, s.Symbol)
		if err == nil && ticker != nil {
			s.logger.Infof("ticker: %s", ticker.String())
			if short {
				return ticker.Buy, nil
			}
			return ticker.Sell, nil
		}

		time.Sleep(1 * time.Second)
	}

	return fixedpoint.Zero, err
}

func (s *Strategy) generateMakerOrder(short bool, budget, price, margin fixedpoint.Value, orderNum int64) ([]types.SubmitOrder, error) {
	marginPrice := price.Mul(margin)
	if !short {
		marginPrice = marginPrice.Neg()
	}

	// TODO: not implement short part yet
	var prices []fixedpoint.Value
	var total fixedpoint.Value
	for i := 0; i < int(orderNum); i++ {
		price = price.Add(marginPrice)
		truncatePrice := s.Market.TruncatePrice(price)

		// need to avoid the price is below 0
		if truncatePrice.Compare(fixedpoint.Zero) <= 0 {
			break
		}

		prices = append(prices, truncatePrice)
		total = total.Add(truncatePrice)
	}

	quantity := fixedpoint.Zero
	l := len(prices) - 1
	for ; l >= 0; l-- {
		if total.IsZero() {
			return nil, fmt.Errorf("total is zero, please check it")
		}

		quantity = budget.Div(total)
		quantity = s.Market.TruncateQuantity(quantity)
		if prices[l].Mul(quantity).Compare(s.Market.MinNotional) > 0 {
			break
		}

		total = total.Sub(prices[l])
	}

	var submitOrders []types.SubmitOrder

	for i := 0; i <= l; i++ {
		submitOrders = append(submitOrders, types.SubmitOrder{
			Symbol:      s.Symbol,
			Market:      s.Market,
			Type:        types.OrderTypeLimit,
			Price:       prices[i],
			Side:        s.makerSide,
			TimeInForce: types.TimeInForceGTC,
			Quantity:    quantity,
			Tag:         orderTag,
			GroupID:     s.OrderGroupID,
		})
	}

	return submitOrders, nil
}
