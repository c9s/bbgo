package dca2

import (
	"context"
	"fmt"

	"github.com/c9s/bbgo/pkg/exchange/retry"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/pkg/errors"
)

func (s *Strategy) placeTakeProfitOrder(ctx context.Context, currentRound Round) error {
	trades, err := s.collector.CollectRoundTrades(ctx, currentRound)
	if err != nil {
		return errors.Wrap(err, "failed to place the take-profit order when collecting round trades")
	}

	roundPosition := types.NewPositionFromMarket(s.Market)
	roundPosition.SetExchangeFeeRate(s.ExchangeSession.ExchangeName, types.ExchangeFee{
		MakerFeeRate: s.ExchangeSession.MakerFeeRate,
		TakerFeeRate: s.ExchangeSession.TakerFeeRate,
	})

	for _, trade := range trades {
		s.logger.Infof("add trade into the position of this round %s", trade.String())
		if trade.FeeProcessing {
			return fmt.Errorf("failed to place the take-profit order because there is a trade's fee not ready")
		}

		roundPosition.AddTrade(trade)
	}
	s.logger.Infof("position of this round before place the take-profit order: %s", roundPosition.String())

	takeProfitPrice := s.Market.TruncatePrice(roundPosition.AverageCost.Mul(fixedpoint.One.Add(s.TakeProfitRatio)))
	order := types.SubmitOrder{
		Symbol:      s.Market.Symbol,
		Market:      s.Market,
		Type:        types.OrderTypeLimit,
		Price:       takeProfitPrice,
		Side:        TakeProfitSide,
		TimeInForce: types.TimeInForceGTC,
		Quantity:    roundPosition.GetBase().Abs(),
		Tag:         orderTag,
		GroupID:     s.OrderGroupID,
	}
	s.logger.Infof("placing take-profit order: %s", order.String())

	// verify the volume of order
	bals, err := retry.QueryAccountBalancesUntilSuccessfulLite(ctx, s.ExchangeSession.Exchange)
	if err != nil {
		return errors.Wrapf(err, "failed to query balance to verify")
	}

	bal, exist := bals[s.Market.BaseCurrency]
	if !exist {
		return fmt.Errorf("there is no %s in the balances %+v", s.Market.BaseCurrency, bals)
	}

	if bal.Available.Compare(order.Quantity) < 0 {
		return fmt.Errorf("the available base balance (%s) is not enough for the order (%s)", bal.Available.String(), order.Quantity.String())
	}

	createdOrders, err := s.OrderExecutor.SubmitOrders(ctx, order)
	if err != nil {
		return err
	}

	for _, createdOrder := range createdOrders {
		s.logger.Infof("submit take-profit order successfully: %s", createdOrder.String())
	}

	return nil
}
