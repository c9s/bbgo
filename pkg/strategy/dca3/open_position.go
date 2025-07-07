package dca3

import (
	"context"
	"fmt"
	"time"

	"github.com/c9s/bbgo/pkg/exchange/retry"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func (s *Strategy) openPosition(ctx context.Context) error {
	s.logger.Info("try to open position stage")
	if s.nextRoundPaused {
		return fmt.Errorf("nextRoundPaused is set to true, not placing open-position orders")
	}

	if time.Now().Before(s.startTimeOfNextRound) {
		return fmt.Errorf("startTimeOfNextRound (%s) is not reached yet, not placing open-position orders", s.startTimeOfNextRound.String())
	}

	// validate the stage by orders
	currentRound, err := s.collector.CollectCurrentRound(ctx, recoverSinceLimit)
	if err != nil {
		return fmt.Errorf("failed to collect current round: %w", err)
	}

	for _, order := range currentRound.OpenPositionOrders {
		if types.IsActiveOrder(order) {
			return fmt.Errorf("there is at least one open-position order #%d is still active, it means we should not open position, please check it", order.OrderID)
		}
	}

	for _, order := range currentRound.TakeProfitOrders {
		if types.IsActiveOrder(order) {
			return fmt.Errorf("there is at least one take-profit order #%d is still active, it means we should not open position, please check it", order.OrderID)
		}
	}

	s.logger.Info("place open-position orders")
	if err := s.placeOpenPositionOrders(ctx); err != nil {
		return fmt.Errorf("failed to place open-position orders: %w", err)
	}

	return nil
}

func (s *Strategy) placeOpenPositionOrders(ctx context.Context) error {
	// get the best ask price to place open position orders
	ticker, err := retry.QueryTickerUntilSuccessful(ctx, s.ExchangeSession.Exchange, s.Symbol)
	if err != nil {
		return fmt.Errorf("failed to get best price: %w", err)
	}

	if ticker.Sell.IsZero() {
		return fmt.Errorf("best ask price is zero, cannot place open position orders")
	}

	// according to the settings to generate the open position orders
	orders, err := generateOpenPositionOrders(s.Market, s.QuoteInvestment, s.ProfitStats.TotalProfit, ticker.Sell, s.PriceDeviation, s.MaxOrderCount, s.OrderGroupID)
	if err != nil {
		return fmt.Errorf("failed to generate open position orders: %w", err)
	}

	createdOrders, err := s.OrderExecutor.SubmitOrders(ctx, orders...)
	if err != nil {
		return fmt.Errorf("failed to submit open position orders: %w", err)
	}

	debugOrders(s.logger, "CREATED", createdOrders)

	return nil
}

func generateOpenPositionOrders(market types.Market, quoteInvestment, profit, price, priceDeviation fixedpoint.Value, orderNum int64, orderGroupID uint32) ([]types.SubmitOrder, error) {
	quoteForOneOrder := market.TruncatePrice(quoteInvestment.Div(fixedpoint.NewFromInt(orderNum)))
	if quoteForOneOrder.Compare(market.MinNotional) < 0 {
		return nil, fmt.Errorf("the quote for one order (%f) is under the min notional (%f)", quoteForOneOrder.Float64(), market.MinNotional.Float64())
	}

	factor := fixedpoint.One.Sub(priceDeviation)
	profit = market.TruncatePrice(profit)

	// calculate the valid prices
	prices := make([]fixedpoint.Value, orderNum)
	for i := 0; i < int(orderNum); i++ {
		if i > 0 {
			price = price.Mul(factor)
		}
		price = market.TruncatePrice(price)
		if price.Compare(market.MinPrice) < 0 {
			return nil, fmt.Errorf("the price for order(#%d) is under the min price (%f), price: %f", i+1, market.MinPrice.Float64(), price.Float64())
		}

		prices[i] = price
	}

	quantities := make([]fixedpoint.Value, orderNum)
	// calculate the quantity for each price
	for i, p := range prices {
		quote := quoteForOneOrder
		// the first order need to add the profit to increase the funding usage, the rest orders only use the quoteForOneOrder
		if i == 0 {
			quote = market.TruncatePrice(quote.Add(profit))
		}
		quantities[i] = market.TruncateQuantity(quote.Div(p))

		if quantities[i].Compare(market.MinQuantity) < 0 {
			return nil, fmt.Errorf("the quantity for order(#%d) is under the min quantity (%f), quote: %f, price: %f, quantity: %f", i+1, market.MinQuantity.Float64(), quote.Float64(), p.Float64(), quantities[i].Float64())
		}
	}

	side := types.SideTypeBuy

	var submitOrders []types.SubmitOrder
	for i := 0; i < int(orderNum); i++ {
		submitOrders = append(submitOrders, types.SubmitOrder{
			Symbol:      market.Symbol,
			Market:      market,
			Type:        types.OrderTypeLimit,
			Price:       prices[i],
			Side:        side,
			TimeInForce: types.TimeInForceGTC,
			Quantity:    quantities[i],
			Tag:         orderTag,
			GroupID:     orderGroupID,
		})
	}

	return submitOrders, nil
}

func (s *Strategy) cancelOpenPositionOrders(ctx context.Context) error {
	s.logger.Info("try to cancel open-position orders")
	var activeOpenPositionOrders types.OrderSlice
	orders := s.OrderExecutor.ActiveMakerOrders().Orders()
	for _, order := range orders {
		if types.IsActiveOrder(order) && order.Side == OpenPositionSide {
			activeOpenPositionOrders = append(activeOpenPositionOrders, order)
		}
	}

	if len(activeOpenPositionOrders) == 0 {
		s.logger.Warn("no active open-position orders to update, nothing to do")
		return nil
	}

	return s.OrderExecutor.GracefulCancel(ctx, activeOpenPositionOrders...)
}
