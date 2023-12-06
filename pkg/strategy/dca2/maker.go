package dca2

import (
	"context"
	"fmt"
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func (s *Strategy) placeDCAOrders(ctx context.Context) error {
	s.logger.Infof("[DCA] start placing maker orders")
	price, err := s.getBestPriceUntilSuccess(ctx, s.Short)
	if err != nil {
		return err
	}

	orders, err := s.generateDCAOrders(s.Short, s.Budget, price, s.PriceDeviation, s.MaxOrderNum)
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

func (s *Strategy) generateDCAOrders(short bool, budget, price, priceDeviation fixedpoint.Value, maxOrderNum int64) ([]types.SubmitOrder, error) {
	// TODO: not implement short part yet
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

	notional, orderNum := calculateDCAMakerOrderNotionalAndNum(s.Market, short, budget, prices)
	if orderNum == 0 {
		return nil, fmt.Errorf("failed to calculate DCA maker order notional and num, price: %s, budget: %s", price, budget)
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

// calculateDCAMakerNotionalAndNum will calculate the notional and num of DCA orders
// DCA2 is notional-based, every order has the same notional
func calculateDCAMakerOrderNotionalAndNum(market types.Market, short bool, budget fixedpoint.Value, prices []fixedpoint.Value) (fixedpoint.Value, int) {
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
