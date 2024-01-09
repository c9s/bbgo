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
	price, err := getBestPriceUntilSuccess(ctx, s.Session.Exchange, s.Symbol)
	if err != nil {
		return err
	}

	orders, err := generateOpenPositionOrders(s.Market, s.ProfitStats.QuoteInvestment, price, s.PriceDeviation, s.MaxOrderCount, s.OrderGroupID)
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

func getBestPriceUntilSuccess(ctx context.Context, ex types.Exchange, symbol string) (fixedpoint.Value, error) {
	ticker, err := retry.QueryTickerUntilSuccessful(ctx, ex, symbol)
	if err != nil {
		return fixedpoint.Zero, err
	}

	return ticker.Sell, nil
}

func generateOpenPositionOrders(market types.Market, quoteInvestment, price, priceDeviation fixedpoint.Value, maxOrderCount int64, orderGroupID uint32) ([]types.SubmitOrder, error) {
	factor := fixedpoint.One.Sub(priceDeviation)

	// calculate all valid prices
	var prices []fixedpoint.Value
	for i := 0; i < int(maxOrderCount); i++ {
		if i > 0 {
			price = price.Mul(factor)
		}
		price = market.TruncatePrice(price)
		if price.Compare(market.MinPrice) < 0 {
			break
		}

		prices = append(prices, price)
	}

	notional, orderNum := calculateNotionalAndNumOrders(market, quoteInvestment, prices)
	if orderNum == 0 {
		return nil, fmt.Errorf("failed to calculate notional and num of open position orders, price: %s, quote investment: %s", price, quoteInvestment)
	}

	side := types.SideTypeBuy

	var submitOrders []types.SubmitOrder
	for i := 0; i < orderNum; i++ {
		quantity := market.TruncateQuantity(notional.Div(prices[i]))
		submitOrders = append(submitOrders, types.SubmitOrder{
			Symbol:      market.Symbol,
			Market:      market,
			Type:        types.OrderTypeLimit,
			Price:       prices[i],
			Side:        side,
			TimeInForce: types.TimeInForceGTC,
			Quantity:    quantity,
			Tag:         orderTag,
			GroupID:     orderGroupID,
		})
	}

	return submitOrders, nil
}

// calculateNotionalAndNumOrders calculates the notional and num of open position orders
// DCA2 is notional-based, every order has the same notional
func calculateNotionalAndNumOrders(market types.Market, quoteInvestment fixedpoint.Value, prices []fixedpoint.Value) (fixedpoint.Value, int) {
	for num := len(prices); num > 0; num-- {
		notional := quoteInvestment.Div(fixedpoint.NewFromInt(int64(num)))
		if notional.Compare(market.MinNotional) < 0 {
			continue
		}

		maxPriceIdx := 0
		quantity := market.TruncateQuantity(notional.Div(prices[maxPriceIdx]))
		if quantity.Compare(market.MinQuantity) < 0 {
			continue
		}

		return notional, num
	}

	return fixedpoint.Zero, 0
}
