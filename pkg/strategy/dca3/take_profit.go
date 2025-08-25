package dca3

import (
	"context"
	"fmt"
	"time"

	"github.com/c9s/bbgo/pkg/exchange/retry"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/pkg/errors"
)

func (s *Strategy) startTakeProfitStage(ctx context.Context) error {
	if s.Position.GetBase().Abs().Compare(s.Market.MinQuantity) < 0 {
		return fmt.Errorf("position base (%s) is less than min quantity (%s), not placing take-profit order", s.Position.GetBase().String(), s.Market.MinQuantity.String())
	}
	s.logger.Info("try to place take-profit order stage")

	currentRound, err := s.collector.CollectCurrentRound(ctx, recoverSinceLimit)
	if err != nil {
		return fmt.Errorf("failed to collect current round: %w", err)
	}

	for _, order := range currentRound.TakeProfitOrders {
		if types.IsActiveOrder(order) {
			return fmt.Errorf("there is at least one take-profit order #%d is still active, it means we should not place take-profit order again, please check it", order.OrderID)
		}
	}

	// make sure the executed quantity of open-position orders is enough
	var executedQuantity fixedpoint.Value
	for _, order := range currentRound.OpenPositionOrders {
		executedQuantity = executedQuantity.Add(order.ExecutedQuantity)
	}

	if executedQuantity.Compare(s.Market.MinQuantity) < 0 {
		return fmt.Errorf("executed quantity (%f) is less than min quantity (%f), not placing take-profit order", executedQuantity.Float64(), s.Market.MinQuantity.Float64())
	}

	if err := s.placeTakeProfitOrder(ctx, currentRound); err != nil {
		return fmt.Errorf("failed to place take-profit order: %w", err)
	}

	return nil
}

func (s *Strategy) placeTakeProfitOrder(ctx context.Context, currentRound Round) error {
	trades, err := s.collector.CollectRoundTrades(ctx, currentRound)
	if err != nil {
		return errors.Wrap(err, "failed to place the take-profit order when collecting round trades")
	}

	roundPosition := types.NewPositionFromMarket(s.Market)
	if s.ExchangeSession.MakerFeeRate.Sign() > 0 || s.ExchangeSession.TakerFeeRate.Sign() > 0 {
		roundPosition.SetExchangeFeeRate(s.ExchangeSession.ExchangeName, types.ExchangeFee{
			MakerFeeRate: s.ExchangeSession.MakerFeeRate,
			TakerFeeRate: s.ExchangeSession.TakerFeeRate,
		})
	}

	for i, trade := range trades {
		s.logger.Infof("add #%d trade into the position of this round %s", i, trade.String())
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

func (s *Strategy) cancelTakeProfitOrders(ctx context.Context) error {
	s.logger.Info("try to cancel take-profit orders")
	var activeTakeProfitOrders types.OrderSlice
	orders := s.OrderExecutor.ActiveMakerOrders().Orders()
	for _, order := range orders {
		if types.IsActiveOrder(order) && order.Side == TakeProfitSide {
			activeTakeProfitOrders = append(activeTakeProfitOrders, order)
		}
	}

	if len(activeTakeProfitOrders) == 0 {
		s.logger.Warn("no active take-profit orders to update, nothing to do")
		return nil
	}

	if err := s.OrderExecutor.GracefulCancel(ctx, activeTakeProfitOrders...); err != nil {
		return fmt.Errorf("failed to cancel existing take-profit orders: %w", err)
	}

	return nil
}

func (s *Strategy) updateTakeProfitOrder(ctx context.Context) error {
	s.logger.Info("try to update take-profit order")
	if err := s.cancelTakeProfitOrders(ctx); err != nil {
		return fmt.Errorf("failed to cancel existing take-profit orders: %w", err)
	}

	s.logger.Info("try to place new take-profit order")

	currentRound, err := s.collector.CollectCurrentRound(ctx, recoverSinceLimit)
	if err != nil {
		return fmt.Errorf("failed to collect current round: %w", err)
	}

	// make sure the executed quantity of open-position orders is enough
	var executedQuantity fixedpoint.Value
	for _, order := range currentRound.OpenPositionOrders {
		executedQuantity = executedQuantity.Add(order.ExecutedQuantity)
	}

	if executedQuantity.Compare(s.Market.MinQuantity) < 0 {
		return fmt.Errorf("executed quantity (%f) is less than min quantity (%f), not placing take-profit order", executedQuantity.Float64(), s.Market.MinQuantity.Float64())
	}

	return s.placeTakeProfitOrder(ctx, currentRound)
}

func (s *Strategy) finishTakeProfitStage(ctx context.Context) error {
	s.logger.Info("try to finish take-profit stage")
	if s.OrderExecutor.ActiveMakerOrders().NumOfOrders() > 0 {
		return fmt.Errorf("there are still active orders so we can't finish take-profit stage, please check it")
	}

	// cancel all orders
	if err := s.OrderExecutor.GracefulCancel(ctx); err != nil {
		return fmt.Errorf("failed to cancel all orders: %w", err)
	}

	// wait 3 seconds to avoid position not update
	time.Sleep(3 * time.Second)

	s.logger.Info("[State] finishTakeProfitStage - start resetting position and calculate quote investment for next round")

	// update profit stats
	if err := s.UpdateProfitStatsUntilSuccessful(ctx); err != nil {
		s.logger.WithError(err).Warn("failed to calculate and emit profit")
	}

	// reset position and open new round for profit stats before position opening
	s.Position.Reset()

	// emit position
	s.OrderExecutor.TradeCollector().EmitPositionUpdate(s.Position)

	// set the start time of the next round
	s.startTimeOfNextRound = time.Now().Add(s.CoolDownInterval.Duration())

	return nil
}
