package xfundingv2

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/exchange/retry"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type PendingRound struct {
	Round         *ArbitrageRound `json:"round"`
	RetryCount    int             `json:"retryCount"`
	LastRetryTime time.Time       `json:"lastRetryTime"`
}

func (s *Strategy) processPendingRounds(ctx context.Context, currentTime time.Time) {
	var pendingRounds []*PendingRound
	for _, pendingRound := range s.PendingRounds {
		// moving round out from the pending list
		if pendingRound.RetryCount >= s.MaxPendingRoundRetry {
			delete(s.PendingRounds, pendingRound.Round.SpotSymbol())
			continue
		}
		// the pending round is over the grace period, it is ready for another retry
		if pendingRound.LastRetryTime.IsZero() || currentTime.Sub(pendingRound.LastRetryTime) >= s.PendingRoundGracePeriod.Duration() {
			pendingRounds = append(pendingRounds, pendingRound)
		}
	}
	var processedRounds []*PendingRound
	if s.FeeSymbol != "" {
		// prepare fee asset for the pending rounds
		var allRounds []*ArbitrageRound
		for _, pendingRound := range pendingRounds {
			// calculate the fee asset required for the round
			if err := s.calculateRoundFeeAsset(pendingRound.Round); err != nil {
				s.logger.WithError(err).Errorf(
					"failed to prepare fee asset for round: %s",
					pendingRound.Round,
				)
				pendingRound.RetryCount++
				pendingRound.LastRetryTime = currentTime
				continue
			}
			processedRounds = append(processedRounds, pendingRound)
			allRounds = append(allRounds, pendingRound.Round)
		}

		if err := s.acquireFeeAssetAndTransfer(ctx, allRounds); err != nil {
			s.logger.WithError(err).Error("failed to acquire fee asset and transfer for pending rounds")
			for _, pendingRound := range pendingRounds {
				pendingRound.RetryCount++
				pendingRound.LastRetryTime = currentTime
			}
			return
		}
		// We set fee average cost after acquiring fee asset and transferring.
		// Because the fee average cost may change after the fee asset acquisition.
		if feePosition, ok := s.SpotPositions[s.FeeSymbol]; ok {
			for _, round := range allRounds {
				feeAvgCost := feePosition.AverageCost
				round.SetAvgFeeCost(s.FeeSymbol, feeAvgCost)
			}
		}
	} else {
		// fee asset is not required, adding all pending rounds to the next step
		for _, pendingRound := range pendingRounds {
			processedRounds = append(processedRounds, pendingRound)
		}
	}

	// start the processed rounds and move them to active round list
	for _, pendingRound := range processedRounds {
		round := pendingRound.Round
		if err := round.Start(
			ctx,
			s.spotSession,
			s.futuresSession,
			currentTime); err != nil {
			s.logger.WithError(err).Errorf(
				"failed to start round after fee asset preparation: %s",
				round,
			)
			pendingRound.RetryCount++
			pendingRound.LastRetryTime = currentTime
			continue
		}
		// move to active round list
		s.ActiveRounds[round.SpotSymbol()] = round
		delete(s.PendingRounds, round.SpotSymbol())
		spotOrderBook, futuresOrderBook, _ := s.getOrderBooks(
			round.SpotSymbol(),
			round.FuturesSymbol(),
		)
		bbgo.Notify("🚀 Round started: %s", round.SpotSymbol(),
			round.NewNotification(
				spotOrderBook, futuresOrderBook,
			),
		)
	}
}

func (s *Strategy) acquireFeeAssetAndTransfer(ctx context.Context, rounds []*ArbitrageRound) error {
	if s.DryRun {
		if len(rounds) > 0 {
			s.logger.Infof(
				"[acquireFeeAssetAndTransfer] dry run mode, would have acquired fee asset and transfer: %+v",
				rounds,
			)
		}
		return nil
	}
	var requiredSpotFeeAmount, requiredFuturesFeeAmount fixedpoint.Value
	for _, round := range rounds {
		roundSpotFee, roundFuturesFee := round.RequiredFeeAssetAmounts()
		requiredSpotFeeAmount = requiredSpotFeeAmount.Add(roundSpotFee)
		requiredFuturesFeeAmount = requiredFuturesFeeAmount.Add(roundFuturesFee)
	}
	spotAccount := s.spotSession.GetAccount()
	futuresAccount := s.futuresSession.GetAccount()
	market, _ := s.spotSession.Market(s.FeeSymbol)
	spotFeeBalance, _ := spotAccount.Balance(market.BaseCurrency)
	futuresFeeBalance, _ := futuresAccount.Balance(market.BaseCurrency)
	spotDeficit := requiredSpotFeeAmount.Sub(spotFeeBalance.Available)
	futuresDeficit := requiredFuturesFeeAmount.Sub(futuresFeeBalance.Available)
	var buyQuantity, transferAmount fixedpoint.Value
	var transferDirection types.TransferDirection
	if spotDeficit.Add(futuresDeficit).Sign() >= 0 {
		// this case means the total fee asset balance is not sufficient
		// need to buy and transfer in the fee asset to cover the deficit
		buyQuantity = spotDeficit.Add(futuresDeficit) // must be positive or zero
		transferAmount = futuresDeficit.Abs()
		if futuresDeficit.Sign() > 0 {
			transferDirection = types.TransferIn
		} else if futuresDeficit.Sign() < 0 {
			transferDirection = types.TransferOut
		}
	} else {
		// this case means the total fee asset balance is sufficient
		if futuresDeficit.Sign() > 0 {
			transferAmount = futuresDeficit
			transferDirection = types.TransferIn
		}
		if spotDeficit.Sign() > 0 {
			transferAmount = spotDeficit
			transferDirection = types.TransferOut
		}
	}
	if !buyQuantity.IsZero() {
		orderBook := s.spotOrderBooks[s.FeeSymbol]
		bestAsk, ok := orderBook.BestAsk()
		if !ok {
			return fmt.Errorf("no ask price available for %s", s.FeeSymbol)
		}
		bestAskPrice := bestAsk.Price
		buyQuantity = fixedpoint.Max(buyQuantity, market.MinNotional.Div(bestAskPrice))
		buyQuantity = market.TruncateQuantity(buyQuantity)
		orderForm := types.SubmitOrder{
			Symbol:   s.FeeSymbol,
			Side:     types.SideTypeBuy,
			Type:     types.OrderTypeMarket,
			Quantity: buyQuantity,
		}
		if market.IsDustQuantity(buyQuantity, bestAskPrice) {
			orderForm.Quantity = market.MinNotional.Mul(fixedpoint.NewFromFloat(1.05)).Div(bestAskPrice)
		}
		orderExecutor, found := s.spotGeneralOrderExecutors[s.FeeSymbol]
		if !found {
			return fmt.Errorf("no order executor found for fee symbol %s", s.FeeSymbol)
		}
		s.logger.Debugf("fee order form: %+v", orderForm)
		if createdOrders, err := orderExecutor.SubmitOrders(ctx, orderForm); err != nil {
			return fmt.Errorf("failed to buy fee asset for pending rounds: %w", err)
		} else {
			timedCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
			defer cancel()

			createdOrder := createdOrders[0]
			s.logger.Debugf("fee order created: %+v", createdOrder)
			if service, ok := s.spotSession.Exchange.(types.ExchangeOrderQueryService); ok {
				_, err := retry.QueryOrderUntilFilled(timedCtx, service, createdOrder.AsQuery())
				if err != nil {
					return fmt.Errorf("failed to wait for fee order to be filled: %w", err)
				}
			} else {
				// simple hack to wait for the market order to be filled
				if !bbgo.IsBackTesting {
					time.Sleep(5 * time.Second)
				}
			}
		}
	}
	if !transferAmount.IsZero() {
		if err := s.futuresService.TransferFuturesAccountAsset(ctx, market.BaseCurrency, transferAmount, transferDirection); err != nil {
			return fmt.Errorf("failed to transfer %s %s for pending rounds fee preparation: %w", transferAmount, market.BaseCurrency, err)
		}
	}
	return nil
}

func (s *Strategy) calculateRoundFeeAsset(round *ArbitrageRound) error {
	if !round.SpotFeeAssetAmount().IsZero() || !round.FuturesFeeAssetAmount().IsZero() {
		// already calculated, no need to calculate again
		return nil
	}
	feeOrderBook, ok := s.spotOrderBooks[s.FeeSymbol]
	if !ok {
		return nil
	}

	// overestimate the fee asset amount with taker fee rate
	spotFeeRate := s.spotSession.TakerFeeRate
	futuresFeeRate := s.futuresSession.TakerFeeRate

	// calculate the spot/futures leg fee assets
	var spotFeeAmount, futuresFeeAmount fixedpoint.Value
	targetPosition := round.TargetPosition()
	positionSize := targetPosition.Abs()

	// get fee asset price (we need to buy it, so use the sell/ask side)
	feeSellBook := feeOrderBook.SideBook(types.SideTypeSell)
	var spotBasePrice, futuresBasePrice fixedpoint.Value
	if targetPosition.Sign() > 0 {
		// long spot (buy at ask side), short futures (sell at bid side)
		spotSellBook := s.spotOrderBooks[round.SpotSymbol()].SideBook(types.SideTypeSell)
		futuresBuyBook := s.futuresOrderBooks[round.FuturesSymbol()].SideBook(types.SideTypeBuy)

		spotBasePrice = spotSellBook.AverageDepthPrice(positionSize)
		futuresBasePrice = futuresBuyBook.AverageDepthPrice(positionSize)
		if spotBasePrice.IsZero() || futuresBasePrice.IsZero() {
			return errors.New("order book data is not ready yet")
		}
	} else {
		// short spot (sell at bid side), long futures (buy at ask side)
		spotBuyBook := s.spotOrderBooks[round.SpotSymbol()].SideBook(types.SideTypeBuy)
		futuresSellBook := s.futuresOrderBooks[round.FuturesSymbol()].SideBook(types.SideTypeSell)

		spotBasePrice = spotBuyBook.AverageDepthPrice(positionSize)
		futuresBasePrice = futuresSellBook.AverageDepthPrice(positionSize)
		if spotBasePrice.IsZero() || futuresBasePrice.IsZero() {
			return errors.New("order book data is not ready yet")
		}
	}
	spotFeeInQuote := spotBasePrice.Mul(positionSize).Mul(spotFeeRate)
	futuresFeeInQuote := futuresBasePrice.Mul(positionSize).Mul(futuresFeeRate)
	totalFeeInQuote := spotFeeInQuote.Add(futuresFeeInQuote)
	feeAssetPrice := feeSellBook.AverageDepthPriceByQuote(totalFeeInQuote, 0)
	spotFeeAmount = spotFeeInQuote.Div(feeAssetPrice)
	futuresFeeAmount = futuresFeeInQuote.Div(feeAssetPrice)

	// multiply by 2 -> for entry and exit
	spotFeeAmount = spotFeeAmount.Mul(fixedpoint.Two)
	futuresFeeAmount = futuresFeeAmount.Mul(fixedpoint.Two)

	round.SetSpotFeeAssetAmount(spotFeeAmount)
	round.SetFuturesFeeAssetAmount(futuresFeeAmount)

	return nil
}
