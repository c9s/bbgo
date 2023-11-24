package dca2

import (
	"context"
	"fmt"

	"github.com/c9s/bbgo/pkg/exchange/retry"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type cancelOrdersByGroupIDApi interface {
	CancelOrdersByGroupID(ctx context.Context, groupID int64) ([]types.Order, error)
}

func (s *Strategy) placeOpenPositionOrders(ctx context.Context) error {
	s.logger.Infof("[DCA] start placing open position orders")
	price, err := getBestPriceUntilSuccess(ctx, s.Session.Exchange, s.Symbol, s.Short)
	if err != nil {
		return err
	}

	orders, err := s.generateOpenPositionOrders(s.Short, s.Budget, price, s.PriceDeviation, s.MaxOrderNum)
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

func getBestPriceUntilSuccess(ctx context.Context, ex types.Exchange, symbol string, short bool) (fixedpoint.Value, error) {
	ticker, err := retry.QueryTickerUntilSuccessful(ctx, ex, symbol)
	if err != nil {
		return fixedpoint.Zero, err
	}

	if short {
		return ticker.Buy, nil
	} else {
		return ticker.Sell, nil
	}
}

func (s *Strategy) generateOpenPositionOrders(short bool, budget, price, priceDeviation fixedpoint.Value, maxOrderNum int64) ([]types.SubmitOrder, error) {
	factor := fixedpoint.One.Sub(priceDeviation)
	if short {
		factor = fixedpoint.One.Add(priceDeviation)
	}

	// calculate all valid prices
	var prices []fixedpoint.Value
	for i := 0; i < int(maxOrderNum); i++ {
		if i > 0 {
			price = price.Mul(factor)
		}
		price = s.Market.TruncatePrice(price)
		if price.Compare(s.Market.MinPrice) < 0 {
			break
		}

		prices = append(prices, price)
	}

	notional, orderNum := calculateNotionalAndNum(s.Market, short, budget, prices)
	if orderNum == 0 {
		return nil, fmt.Errorf("failed to calculate notional and num of open position orders, price: %s, budget: %s", price, budget)
	}

	var submitOrders []types.SubmitOrder
	for i := 0; i < orderNum; i++ {
		quantity := s.Market.TruncateQuantity(notional.Div(prices[i]))
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

// calculateNotionalAndNum calculates the notional and num of open position orders
// DCA2 is notional-based, every order has the same notional
func calculateNotionalAndNum(market types.Market, short bool, budget fixedpoint.Value, prices []fixedpoint.Value) (fixedpoint.Value, int) {
	for num := len(prices); num > 0; num-- {
		notional := budget.Div(fixedpoint.NewFromInt(int64(num)))
		if notional.Compare(market.MinNotional) < 0 {
			continue
		}

		maxPriceIdx := 0
		if short {
			maxPriceIdx = num - 1
		}
		quantity := market.TruncateQuantity(notional.Div(prices[maxPriceIdx]))
		if quantity.Compare(market.MinQuantity) < 0 {
			continue
		}

		return notional, num
	}

	return fixedpoint.Zero, 0
}

func (s *Strategy) cancelOpenPositionOrders(ctx context.Context) error {
	s.logger.Info("[DCA] cancel open position orders")
	e, ok := s.Session.Exchange.(cancelOrdersByGroupIDApi)
	if ok {
		cancelledOrders, err := e.CancelOrdersByGroupID(ctx, int64(s.OrderGroupID))
		if err != nil {
			return err
		}

		for _, cancelledOrder := range cancelledOrders {
			s.logger.Info("CANCEL ", cancelledOrder.String())
		}
	} else {
		if err := s.OrderExecutor.ActiveMakerOrders().GracefulCancel(ctx, s.Session.Exchange); err != nil {
			return err
		}
	}

	return nil
}
