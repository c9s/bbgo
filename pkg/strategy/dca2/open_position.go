package dca2

import (
	"context"
	"fmt"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/exchange/retry"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type cancelOrdersByGroupIDApi interface {
	CancelOrdersByGroupID(ctx context.Context, groupID int64) ([]types.Order, error)
}

func (s *Strategy) placeOpenPositionOrders(ctx context.Context) error {
	s.logger.Infof("start placing open position orders")
	price, err := getBestPriceUntilSuccess(ctx, s.ExchangeSession.Exchange, s.Symbol)
	if err != nil {
		return err
	}

	orders, err := generateOpenPositionOrders(s.Market, s.EnableQuoteInvestmentReallocate, s.QuoteInvestment, s.ProfitStats.TotalProfit, price, s.PriceDeviation, s.MaxOrderCount, s.OrderGroupID)
	if err != nil {
		return err
	}

	createdOrders, err := s.OrderExecutor.SubmitOrders(ctx, orders...)
	if err != nil {
		return err
	}

	s.debugOrders(createdOrders)

	if s.DevMode != nil && s.DevMode.Enabled && s.DevMode.IsNewAccount {
		if len(createdOrders) > 0 {
			s.ProfitStats.FromOrderID = createdOrders[0].OrderID
		}

		for _, createdOrder := range createdOrders {
			if s.ProfitStats.FromOrderID > createdOrder.OrderID {
				s.ProfitStats.FromOrderID = createdOrder.OrderID
			}
		}

		s.DevMode.IsNewAccount = false
		bbgo.Sync(ctx, s)
	}

	return nil
}

func getBestPriceUntilSuccess(ctx context.Context, ex types.Exchange, symbol string) (fixedpoint.Value, error) {
	ticker, err := retry.QueryTickerUntilSuccessful(ctx, ex, symbol)
	if err != nil {
		return fixedpoint.Zero, err
	}

	return ticker.Sell, nil
}

func generateOpenPositionOrders(market types.Market, enableQuoteInvestmentReallocate bool, quoteInvestment, profit, price, priceDeviation fixedpoint.Value, maxOrderCount int64, orderGroupID uint32) ([]types.SubmitOrder, error) {
	factor := fixedpoint.One.Sub(priceDeviation)
	profit = market.TruncatePrice(profit)

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

	if !enableQuoteInvestmentReallocate && orderNum != int(maxOrderCount) {
		return nil, fmt.Errorf("failed to generate open-position orders due to the orders may be under min notional or quantity")
	}

	side := types.SideTypeBuy

	var submitOrders []types.SubmitOrder
	for i := 0; i < orderNum; i++ {
		var quantity fixedpoint.Value
		// all the profit will use in the first order
		if i == 0 {
			quantity = market.TruncateQuantity(notional.Add(profit).Div(prices[i]))
		} else {
			quantity = market.TruncateQuantity(notional.Div(prices[i]))
		}
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

		return market.TruncatePrice(notional), num
	}

	return fixedpoint.Zero, 0
}
