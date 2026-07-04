package xfundingv2

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/slack-go/slack"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/exchange/batch"
	"github.com/c9s/bbgo/pkg/exchange/binance/binanceapi"
	"github.com/c9s/bbgo/pkg/exchange/retry"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/slack/slackalert"
	"github.com/c9s/bbgo/pkg/types"
)

//go:generate stringer -type=RoundState
type RoundState int

const (
	RoundPending RoundState = iota
	RoundOpening
	RoundReady
	RoundClosing
	RoundClosed
)

type FuturesService interface {
	batch.BinanceFuturesIncomeHistoryService

	TransferFuturesAccountAsset(ctx context.Context, asset string, amount fixedpoint.Value, io types.TransferDirection) error
	QueryPremiumIndex(ctx context.Context, symbol string) (*types.PremiumIndex, error)
	QueryPositionRisk(ctx context.Context, symbol ...string) ([]types.PositionRisk, error)
}

type transferRetry struct {
	Trade     types.Trade             `json:"trade"`
	LastTried time.Time               `json:"lastTried"`
	Direction types.TransferDirection `json:"direction"`
}

type ArbitrageRound struct {
	mu sync.Mutex

	syncState     ArbitrageRoundSyncState
	spotWorker    *TWAPWorker
	futuresWorker *TWAPWorker
	haltedAt      time.Time

	futuresService                                FuturesService
	spotExchangeFeeRates, futuresExchangeFeeRates map[types.ExchangeName]types.ExchangeFee

	retryTransferTickC chan time.Time

	logger     logrus.FieldLogger
	slackAlert slackalert.SlackAlert
}

func NewArbitrageRound(
	fundingRate *types.PremiumIndex,
	spotExchangeName, futuresExchangeName types.ExchangeName,
	minHoldingIntervals, fundingIntervalHours int,
	spotTwap, futuresTwap *TWAPWorker,
	futuresService FuturesService,
	direction types.PositionType,
) *ArbitrageRound {
	policy, err := newDirectionPolicy(direction, spotTwap.Market())
	if err != nil {
		panic(fmt.Sprintf("NewArbitrageRound: %v", err))
	}
	fundingIntervalStart := fundingRate.NextFundingTime.Add(-time.Duration(fundingIntervalHours) * time.Hour)
	fundingIntervalEnd := fundingRate.NextFundingTime.Add(-time.Second)
	return &ArbitrageRound{
		syncState: ArbitrageRoundSyncState{
			Symbol:                      spotTwap.Symbol(),
			TriggeredFundingRate:        fundingRate.LastFundingRate,
			TriggeredSpotTargetPosition: spotTwap.TargetPosition(),
			MinHoldingIntervals:         minHoldingIntervals,
			FundingIntervalHours:        fundingIntervalHours,
			FundingIntervalStart:        fundingIntervalStart,
			FundingIntervalEnd:          fundingIntervalEnd,
			FundingFeeRecords:           make(map[int64]FundingFee),

			SpotExchangeName:    spotExchangeName,
			FuturesExchangeName: futuresExchangeName,
			DirectionPolicy:     policy,
			State:               RoundPending,
			RetryTransfers:      make(map[uint64]*transferRetry),
			SyncedSpotTrades:    make(map[uint64]struct{}),
			SyncedFuturesTrades: make(map[uint64]struct{}),
		},

		spotWorker:         spotTwap,
		futuresWorker:      futuresTwap,
		futuresService:     futuresService,
		retryTransferTickC: make(chan time.Time, 100),
	}
}

func (r *ArbitrageRound) Halt(currentTime time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.haltedAt = currentTime
}

func (r *ArbitrageRound) HaltedAt() time.Time {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.haltedAt
}

func (r *ArbitrageRound) IsHalted() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.isHalted()
}

func (r *ArbitrageRound) isHalted() bool {
	return !r.haltedAt.IsZero()
}

func (r *ArbitrageRound) Resume() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.haltedAt = time.Time{}
}

func (r *ArbitrageRound) SetSpotExchangeFeeRates(fee types.ExchangeFee) {
	r.spotExchangeFeeRates = map[types.ExchangeName]types.ExchangeFee{
		r.syncState.SpotExchangeName: fee,
	}
}

func (r *ArbitrageRound) SetFuturesExchangeFeeRates(fee types.ExchangeFee) {
	r.futuresExchangeFeeRates = map[types.ExchangeName]types.ExchangeFee{
		r.syncState.FuturesExchangeName: fee,
	}
}

func (r *ArbitrageRound) SetAvgFeeCost(feeSymbol string, cost fixedpoint.Value) {
	r.syncState.FeeSymbol = feeSymbol
	r.syncState.AvgFeeCost = cost
}

func (r *ArbitrageRound) SetSpotFeeAssetAmount(amount fixedpoint.Value) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.syncState.SpotFeeAssetAmount = amount
}

func (r *ArbitrageRound) SpotFeeAssetAmount() fixedpoint.Value {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.syncState.SpotFeeAssetAmount
}

func (r *ArbitrageRound) SetFuturesFeeAssetAmount(amount fixedpoint.Value) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.syncState.FuturesFeeAssetAmount = amount
}

func (r *ArbitrageRound) FuturesFeeAssetAmount() fixedpoint.Value {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.syncState.FuturesFeeAssetAmount
}

// RequiredFeeAssetAmount returns the required fee asset amount for the round based on its current state and position.
// The first return value is for the spot leg and the second return value is for the futures leg.
func (r *ArbitrageRound) RequiredFeeAssetAmounts() (fixedpoint.Value, fixedpoint.Value) {
	r.mu.Lock()
	defer r.mu.Unlock()

	halfSpotFee := r.syncState.SpotFeeAssetAmount.Div(fixedpoint.Two)
	halfFuturesFee := r.syncState.FuturesFeeAssetAmount.Div(fixedpoint.Two)
	switch r.syncState.State {
	case RoundPending:
		return r.syncState.SpotFeeAssetAmount, r.syncState.FuturesFeeAssetAmount
	case RoundOpening:
		// calculate the executed ratio
		executedRatio := fixedpoint.Zero
		if !r.spotWorker.TargetPosition().IsZero() {
			executedRatio = r.spotWorker.FilledPosition().
				Abs().
				Div(r.spotWorker.TargetPosition().Abs())
		}
		remainRatio := fixedpoint.Max(
			fixedpoint.One.Sub(executedRatio),
			fixedpoint.Zero,
		)
		// add 1x for the closing leg fee
		remainRatio = remainRatio.Add(fixedpoint.One)
		return halfSpotFee.Mul(remainRatio), halfFuturesFee.Mul(remainRatio)
	case RoundReady, RoundClosing:
		executedRatio := fixedpoint.Zero
		if !r.syncState.TriggeredSpotTargetPosition.IsZero() {
			executedRatio = r.spotWorker.FilledPosition().Abs().Div(r.syncState.TriggeredSpotTargetPosition.Abs())
		}
		remainRatio := fixedpoint.Max(
			fixedpoint.One.Sub(executedRatio),
			fixedpoint.Zero,
		)
		return halfSpotFee.Mul(remainRatio), halfFuturesFee.Mul(remainRatio)
	}
	// the round is closed, no fee asset is required
	return fixedpoint.Zero, fixedpoint.Zero
}

func (r *ArbitrageRound) StartTime() time.Time {
	return r.syncState.StartTime
}

func (r *ArbitrageRound) HasStarted() bool {
	return !r.syncState.StartTime.IsZero()
}

func (r *ArbitrageRound) TriggeredFundingRate() fixedpoint.Value {
	return r.syncState.TriggeredFundingRate
}

func (r *ArbitrageRound) TriggeredTargetPosition() fixedpoint.Value {
	return r.syncState.TriggeredSpotTargetPosition
}

func (r *ArbitrageRound) NumHoldingIntervals(currentTime time.Time) int {
	if r.syncState.StartTime.IsZero() {
		return 0
	}
	intervalDuration := time.Duration(r.syncState.FundingIntervalHours) * time.Hour
	elapsed := currentTime.Sub(r.syncState.FundingIntervalStart)
	if elapsed < 0 {
		return 0
	}
	return int(elapsed / intervalDuration)
}

func (r *ArbitrageRound) MinHoldingIntervals() int {
	return r.syncState.MinHoldingIntervals
}

func (r *ArbitrageRound) TargetPosition() fixedpoint.Value {
	return r.spotWorker.TargetPosition()
}

func (r *ArbitrageRound) LastUpdateTime() time.Time {
	return r.syncState.LastUpdateTime
}

func (r *ArbitrageRound) SetUpdateTime(t time.Time) {
	r.syncState.LastUpdateTime = t
}

func (r *ArbitrageRound) String() string {
	if r.syncState.State != RoundClosing {
		return fmt.Sprintf(
			"ArbitrageRound(symbol=%s, state=%s, spot=%s, futures=%s, startTime=%s)",
			r.spotWorker.Symbol(),
			r.syncState.State,
			r.spotWorker.FilledPosition(),
			r.futuresWorker.FilledPosition(),
			r.syncState.StartTime.Format(time.RFC3339),
		)
	}
	return fmt.Sprintf(
		"ArbitrageRound(symbol=%s, state=%s, spot=%s, futures=%s, closingTime=%s, expectedCloseTime=%s)",
		r.spotWorker.Symbol(),
		r.syncState.State,
		r.spotWorker.FilledPosition(),
		r.futuresWorker.FilledPosition(),
		r.syncState.ClosingTime.Format(time.RFC3339),
		r.syncState.ClosingTime.Add(r.syncState.ClosingDuration.Duration()).Format(time.RFC3339),
	)
}

func (r *ArbitrageRound) SetSlackAlert(alert slackalert.SlackAlert) {
	r.slackAlert = alert
}

func (r *ArbitrageRound) NewCriticalNotification() *roundNotification {
	return &roundNotification{
		ArbitrageRound: r,
		IsCritical:     true,
	}
}

func (r *ArbitrageRound) NewNotification() *roundNotification {
	return &roundNotification{
		ArbitrageRound: r,
		IsCritical:     false,
	}
}

type roundNotification struct {
	*ArbitrageRound
	IsCritical bool
}

func (n *roundNotification) SlackAttachment() slack.Attachment {
	title := fmt.Sprintf("Arbitrage Round %s (%s)", n.SpotSymbol(), n.syncState.State)
	fields := []slack.AttachmentField{
		{
			Title: "Target Spot Position",
			Value: n.syncState.TriggeredSpotTargetPosition.String(),
			Short: true,
		},
		{
			Title: "Funding Interval in Hours",
			Value: fmt.Sprintf("%d", n.syncState.FundingIntervalHours),
			Short: true,
		},
		{
			Title: "Triggered Funding Rate",
			Value: n.syncState.TriggeredFundingRate.Percentage(),
			Short: true,
		},
		{
			Title: "Annualized Funding Rate",
			Value: n.AnnualizedRate().Percentage(),
			Short: true,
		},
	}
	if n.State() == RoundClosing {
		fields = append(fields,
			slack.AttachmentField{
				Title: "Closing Time",
				Value: n.syncState.ClosingTime.Format(time.RFC3339),
				Short: true,
			},
			slack.AttachmentField{
				Title: "Expected Close Time",
				Value: n.syncState.ClosingTime.Add(n.syncState.ClosingDuration.Duration()).Format(time.RFC3339),
				Short: true,
			})
	} else if n.HasStarted() {
		minHoldingHours := n.syncState.MinHoldingIntervals * n.syncState.FundingIntervalHours
		expectedClosingTime := n.syncState.FundingIntervalStart.Add(time.Duration(minHoldingHours) * time.Hour)
		fields = append(fields,
			slack.AttachmentField{
				Title: "Start Time",
				Value: n.syncState.StartTime.Format(time.RFC3339),
				Short: true,
			},
			slack.AttachmentField{
				Title: "Last Update Time",
				Value: n.syncState.LastUpdateTime.Format(time.RFC3339),
				Short: true,
			},
			slack.AttachmentField{
				Title: "Min Holding Intervals",
				Value: fmt.Sprintf("%d", n.syncState.MinHoldingIntervals),
				Short: true,
			},
			slack.AttachmentField{
				Title: "Funding Interval Start",
				Value: n.syncState.FundingIntervalStart.Format(time.RFC3339),
				Short: true,
			},
			slack.AttachmentField{
				Title: "Expected Closing Time",
				Value: expectedClosingTime.Format(time.RFC3339),
			},
		)
	}
	text := "Arbitrage Round Details"
	if n.IsCritical && len(n.slackAlert.Mentions) > 0 {
		text += "cc " + strings.Join(n.slackAlert.Mentions, " ")
	}
	return slack.Attachment{
		Title:  title,
		Text:   text,
		Color:  n.stateColor(),
		Fields: fields,
	}
}

func (r *ArbitrageRound) stateColor() string {
	var color string
	switch r.syncState.State {
	case RoundPending:
		color = "#9C9494"
	case RoundOpening:
		color = "#6FCF97"
	case RoundReady:
		color = "#56CCF2"
	case RoundClosing:
		color = "#F2994A"
	case RoundClosed:
		color = "#B10202"
	default:
		color = "#9C9494"
	}
	return color
}

func (r *ArbitrageRound) TotalFundingIncome() fixedpoint.Value {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.syncState.StartTime.IsZero() {
		return fixedpoint.Zero
	}
	return r.totalFundingIncome()
}

func (r *ArbitrageRound) totalFundingIncome() fixedpoint.Value {
	var totalFunding fixedpoint.Value
	for _, fee := range r.syncState.FundingFeeRecords {
		totalFunding = totalFunding.Add(fee.Amount)
	}
	return totalFunding
}

func (r *ArbitrageRound) SyncFundingFeeRecords(ctx context.Context, currentTime time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.syncFundingFeeRecords(ctx, currentTime)
}

func (r *ArbitrageRound) Orders() map[string][]types.Order {
	r.mu.Lock()
	defer r.mu.Unlock()

	orders := map[string][]types.Order{
		"spot":    r.spotWorker.Executor().AllOrders(),
		"futures": r.futuresWorker.Executor().AllOrders(),
	}

	return orders
}

func (r *ArbitrageRound) Trades() map[string][]types.Trade {
	r.mu.Lock()
	defer r.mu.Unlock()

	trades := map[string][]types.Trade{
		"spot":    r.spotWorker.Executor().AllTrades(),
		"futures": r.futuresWorker.Executor().AllTrades(),
	}

	return trades
}

func (r *ArbitrageRound) HasOrder(orderID uint64) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.hasOrder(orderID)
}

func (r *ArbitrageRound) hasOrder(orderID uint64) bool {
	_, spotExists := r.spotWorker.Executor().GetOrder(orderID)
	_, futuresExists := r.futuresWorker.Executor().GetOrder(orderID)

	return spotExists || futuresExists
}

func (r *ArbitrageRound) syncFundingFeeRecords(ctx context.Context, currentTime time.Time) error {
	if r.syncState.StartTime.IsZero() || r.syncState.StartTime.After(currentTime) {
		return nil
	}

	q := batch.BinanceFuturesIncomeBatchQuery{
		BinanceFuturesIncomeHistoryService: r.futuresService,
	}
	symbol := r.futuresWorker.Symbol()
	dataC, errC := q.Query(ctx, symbol, binanceapi.FuturesIncomeFundingFee, r.syncState.StartTime, currentTime)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case income, ok := <-dataC:
			if !ok {
				return nil
			}
			switch income.IncomeType {
			case binanceapi.FuturesIncomeFundingFee:
				record := FundingFee{
					Asset:  income.Asset,
					Amount: income.Income,
					Txn:    income.TranId,
					Time:   income.Time.Time(),
				}
				r.syncState.FundingFeeRecords[income.TranId] = record
			}
		case err, ok := <-errC:
			if !ok {
				return nil
			}

			return err

		}
	}
}

func (r *ArbitrageRound) Start(ctx context.Context, currentTime time.Time) error {
	if r.syncState.StartTime.IsZero() {
		if currentTime.After(r.syncState.FundingIntervalEnd) {
			// the round is triggered after the funding interval -> error
			return fmt.Errorf(
				"the round is triggered after the funding interval end (%s): %s",
				r.syncState.FundingIntervalEnd.Format(time.RFC3339),
				currentTime.Format(time.RFC3339),
			)
		}
		if err := r.spotWorker.Start(ctx, currentTime); err != nil {
			return fmt.Errorf("failed to start spot worker: %w", err)
		}
		if err := r.futuresWorker.Start(ctx, currentTime); err != nil {
			return fmt.Errorf("failed to start futures worker: %w", err)
		}

		go r.retryTransferWorker(ctx, r.retryTransferTickC)

		r.syncState.StartTime = currentTime
		r.syncState.State = RoundOpening
	}
	return nil
}

func (r *ArbitrageRound) Stop() {
	r.spotWorker.Stop()
	r.futuresWorker.Stop()
	close(r.retryTransferTickC)
}

func (r *ArbitrageRound) retryTransferWorker(ctx context.Context, tickC <-chan time.Time) {
	defer r.logger.Infof("retry transfer worker stopped: %s", r.SpotSymbol())

	for {
		select {
		case <-ctx.Done():
			return
		case currentTime, ok := <-tickC:
			if !ok {
				return
			}
			r.doRetryTransfers(currentTime)
		}
	}
}

func (r *ArbitrageRound) doRetryTransfers(currentTime time.Time) {
	// retry failed transfers if any
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.syncState.State != RoundOpening && r.syncState.State != RoundClosing {
		return
	}

	for _, transfer := range r.syncState.RetryTransfers {
		if r.syncState.RetryDuration == 0 {
			// default retry duration is 10 minutes
			r.syncState.RetryDuration = 10 * time.Minute
		}
		if currentTime.Sub(transfer.LastTried) < r.syncState.RetryDuration {
			continue
		}
		r.logger.Infof("retry transfer (last tried: %s): %s", transfer.LastTried.Format(time.RFC3339), transfer.Trade)
		switch transfer.Direction {
		case types.TransferOut:
			// the round should be in closing state
			r.handleFuturesTradeForClose(transfer.Trade, currentTime)
		case types.TransferIn:
			// the round should be in opening state
			r.handleSpotTradeForOpen(transfer.Trade, currentTime)
		default:
			r.logger.Warnf("unknown transfer direction for retry: %s", transfer.Direction)
		}
	}
}

func (r *ArbitrageRound) SetRetryDuration(d time.Duration) {
	r.syncState.RetryDuration = d
}

// HandleSpotTrade handles a spot trade, including update filled position, transfer collateral and sync futures position if the round is opening.
func (r *ArbitrageRound) HandleSpotTrade(trade types.Trade, currentTime time.Time) {
	// lock the round to ensure the state is updated correctly when receiving trade updates from spot worker
	r.mu.Lock()
	defer r.mu.Unlock()

	if ok := r.spotWorker.Executor().AddTrade(trade); !ok {
		// the trade does not belong to the spot worker, skip
		return
	}

	activeOrder := r.spotWorker.ActiveOrder()
	if activeOrder.OrderID == trade.OrderID {
		activeOrder.ExecutedQuantity = activeOrder.ExecutedQuantity.Add(trade.Quantity)
	}

	if r.syncState.State == RoundOpening {
		r.logger.Infof("handling spot trade (open): %s", trade)
		r.handleSpotTradeForOpen(trade, currentTime)
	} else {
		// the round is ready or closing, just log the spot trade
		r.logger.Infof("received spot trade: %s", trade)
	}
}

// handleSpotTradeForOpen is the mirror of handleFuturesTradeForClose: during opening, spot is the leader.
// After each spot fill, the futures position is synced to keep delta-neutral and the collateral asset is
// transferred to futures so the futures leg can use it as collateral for its offsetting trade.
func (r *ArbitrageRound) handleSpotTradeForOpen(trade types.Trade, currentTime time.Time) {
	// move the collateral asset received from the spot fill onto the futures
	// account so the futures leg can use it as margin.
	transferAmount := r.syncState.DirectionPolicy.TransferAmountFromSpotTrade(trade)
	if transferAmount.Sign() <= 0 {
		// nothing left to transfer (e.g. fees ate the entire fill); still record sync
		delete(r.syncState.RetryTransfers, trade.ID)
		return
	}
	timedCtx, cancel := context.WithTimeout(
		r.spotWorker.ctx,
		time.Second*20,
	)
	defer cancel()
	asset := r.syncState.DirectionPolicy.CollateralAsset()
	if err := r.futuresService.TransferFuturesAccountAsset(
		timedCtx, asset, transferAmount, types.TransferIn,
	); err != nil {
		if transfer, found := r.syncState.RetryTransfers[trade.ID]; !found {
			bbgo.Notify("🚨 Round spot transfer %s %s failed (%s), retrying: %s",
				transferAmount.String(),
				asset,
				currentTime.Format(time.RFC3339),
				err.Error(),
				trade,
			)
			r.syncState.RetryTransfers[trade.ID] = &transferRetry{
				Trade:     trade,
				LastTried: currentTime,
				Direction: types.TransferIn,
			}
		} else {
			transfer.LastTried = currentTime
		}
		return
	}
	// transfer succeeded, remove from retry list if exists
	delete(r.syncState.RetryTransfers, trade.ID)
	bbgo.Notify("➡️ Transfered %s %s from spot to futures",
		transferAmount.String(),
		asset,
		trade,
	)

	// sync the futures position only after the transfer succeeds.
	if _, found := r.syncState.SyncedSpotTrades[trade.ID]; !found {
		r.syncFuturesPosition(trade)
		r.syncState.SyncedSpotTrades[trade.ID] = struct{}{}
	}
}

// HandleFuturesTrade handles a futures trade, including update filled position, transfer collateral and sync spot position if the round is closing.
func (r *ArbitrageRound) HandleFuturesTrade(trade types.Trade, currentTime time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if ok := r.futuresWorker.Executor().AddTrade(trade); !ok {
		// the trade does not belong to the futures worker, skip
		return
	}

	activeOrder := r.futuresWorker.ActiveOrder()
	if activeOrder.OrderID == trade.OrderID {
		activeOrder.ExecutedQuantity = activeOrder.ExecutedQuantity.Add(trade.Quantity)
	}

	if r.syncState.State == RoundClosing {
		r.logger.Infof("handling future trade (closing): %s", trade)
		r.handleFuturesTradeForClose(trade, currentTime)
	} else {
		// the round is opening or ready, just log the futures trade
		r.logger.Infof("received futures trade: %s", trade)
	}
}

// handleFuturesTradeForClose is the mirror of handleSpotTradeForOpen: during
// closing, futures is the leader. After each futures fill reduces the position,
// the freed collateral asset is transferred back to spot and the spot worker's
// target is advanced so it can execute its offsetting trade with that asset.
func (r *ArbitrageRound) handleFuturesTradeForClose(trade types.Trade, currentTime time.Time) {
	// transfer the freed collateral asset back to spot so the spot leg can use it to close its position.
	transferAmount := r.syncState.DirectionPolicy.TransferAmountFromFuturesTrade(trade)
	asset := r.syncState.DirectionPolicy.CollateralAsset()
	if transferAmount.Sign() <= 0 {
		// nothing left to transfer
		// remove from retry list if exists
		delete(r.syncState.RetryTransfers, trade.ID)
		return
	}

	timedCtx, cancel := context.WithTimeout(
		r.futuresWorker.ctx,
		time.Second*20,
	)
	defer cancel()
	if err := r.futuresService.TransferFuturesAccountAsset(
		timedCtx, asset, transferAmount, types.TransferOut,
	); err != nil {
		if transfer, found := r.syncState.RetryTransfers[trade.ID]; !found {
			bbgo.Notify("🚨 Round futures transfer %s %s failed (%s), retrying: %s",
				transferAmount.String(),
				asset,
				currentTime.Format(time.RFC3339),
				err.Error(),
				trade,
			)
			r.syncState.RetryTransfers[trade.ID] = &transferRetry{
				Trade:     trade,
				LastTried: currentTime,
				Direction: types.TransferOut,
			}
		} else {
			transfer.LastTried = currentTime
		}
		return
	}
	// transfer succeeded, remove from retry list if exists
	delete(r.syncState.RetryTransfers, trade.ID)
	bbgo.Notify("⬅️ Transferred %s %s from futures to spot",
		transferAmount.String(),
		asset,
		trade,
	)

	// sync the spot position only after the transfer succeeds.
	if _, found := r.syncState.SyncedFuturesTrades[trade.ID]; !found {
		r.syncSpotPosition(trade)
		r.syncState.SyncedFuturesTrades[trade.ID] = struct{}{}
	}
}

func (r *ArbitrageRound) SetLogger(logger logrus.FieldLogger) {
	r.logger = logger
}

func (r *ArbitrageRound) SpotSymbol() string {
	return r.spotWorker.Symbol()
}

func (r *ArbitrageRound) FuturesSymbol() string {
	return r.futuresWorker.Symbol()
}

func (r *ArbitrageRound) SpotWorker() *TWAPWorker {
	return r.spotWorker
}

func (r *ArbitrageRound) FuturesWorker() *TWAPWorker {
	return r.futuresWorker
}

func (r *ArbitrageRound) FuturesMarket() types.Market {
	return r.futuresWorker.Market()
}

func (r *ArbitrageRound) State() RoundState {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.syncState.State
}

func (r *ArbitrageRound) SetClosing(currentTime time.Time, duration types.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// During close, futures is the leader: drive its target to zero so it
	// starts reducing the position. The spot target stays where it is and is
	// re-synced after each futures fill in handleFuturesTradeForClose, so spot
	// only trades once the collateral has been transferred back from futures.
	r.futuresWorker.SetTargetPosition(fixedpoint.Zero)
	r.spotWorker.ResetTime(currentTime, duration)
	r.futuresWorker.ResetTime(currentTime, duration)

	r.syncState.State = RoundClosing
	r.syncState.ClosingTime = currentTime
	r.syncState.ClosingDuration = duration
}

// CollateralAsset returns the asset that this round parks on the futures
// account (base for Short, quote for Long).
func (r *ArbitrageRound) CollateralAsset() string {
	return r.syncState.DirectionPolicy.CollateralAsset()
}

func (r *ArbitrageRound) Cleanup(ctx context.Context, orderBook types.OrderBook) error {
	if r.syncState.State != RoundClosed {
		return fmt.Errorf("round is not closed yet: %s", r)
	}
	w := r.futuresWorker
	remaining := w.RemainingQuantity()
	if remaining.IsZero() {
		return nil
	}

	midPrice := getMidPrice(orderBook)
	market := w.Market()
	if market.IsDustQuantity(remaining.Abs(), midPrice) {
		// if the remaining is dust, we adjust it to a much larger amount
		// With `ReduceOnly` flag, theoretically it will reduce the position to zero without creating new position.
		remaining = market.MinNotional.Mul(fixedpoint.Two).Div(midPrice).Round(0, fixedpoint.Up)
	}

	orderOptions := TWAPExecuteOrderOptions{
		DeadlineExceeded: true,
		ReduceOnly:       true,
	}
	createdOrder, err := w.Executor().PlaceOrder(
		remaining.Abs(),
		orderSide(remaining).Reverse(),
		orderBook,
		orderOptions,
	)
	if err != nil || createdOrder == nil {
		return fmt.Errorf("[CloseFuturesPosition] failed to place order to close remaining quantity: %w", err)
	}
	// collect trades for this closing order
	exchange := w.syncState.TWAPExecutor.exchange
	timedCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if _, err := retry.QueryOrderTradesUntilSuccessfulLite(timedCtx, exchange, createdOrder.AsQuery()); err != nil {
		return fmt.Errorf("[CloseFuturesPosition] failed to wait for closing order trades: %w", err)
	}
	risks, err := r.futuresService.QueryPositionRisk(timedCtx, r.FuturesSymbol())
	if err != nil || len(risks) == 0 {
		return fmt.Errorf("[CloseFuturesPosition] failed to query position risk after closing order filled: %w", err)
	}
	risk := risks[0]
	if !risk.PositionAmount.IsZero() {
		return fmt.Errorf("[CloseFuturesPosition] position is not fully closed after closing order filled, remaining position: %s %s", risk.PositionAmount, risk.Symbol)
	}
	return nil
}

func (r *ArbitrageRound) AnnualizedRate() fixedpoint.Value {
	return AnnualizedRate(r.syncState.TriggeredFundingRate, r.syncState.FundingIntervalHours)
}

// Tick is called to tick the underlying spot and futures workers and update the round state
func (r *ArbitrageRound) Tick(currentTime time.Time, spotOrderBook types.OrderBook, futuresOrderBook types.OrderBook) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.syncState.State == RoundPending || r.isHalted() {
		// not started yet or halted, do nothing
		return
	}

	if r.logger == nil {
		r.logger = logrus.WithFields(logrus.Fields{
			"component": "ArbitrageRound",
			"symbol":    r.spotWorker.Symbol(),
		})
	}

	if r.syncState.State == RoundClosed || r.syncState.State == RoundReady {
		return
	}

	select {
	case r.retryTransferTickC <- currentTime:
	default:
		r.logger.Warnf("retry transfer tick channel is full, skipping retry tick at %s", currentTime.Format(time.RFC3339))
	}

	// it's opening or closing, tick the workers
	if err := r.spotWorker.Tick(currentTime, spotOrderBook); err != nil {
		r.logger.
			WithError(err).
			Warnf(
				"failed to tick %s spot worker at %s",
				r.SpotSymbol(), currentTime.Format(time.RFC3339),
			)
	}
	if err := r.futuresWorker.Tick(currentTime, futuresOrderBook); err != nil {
		r.logger.
			WithError(err).
			Warnf(
				"failed to tick %s futures worker at %s",
				r.FuturesSymbol(), currentTime.Format(time.RFC3339),
			)
	}

	// get mid price
	spotMidPrice := getMidPrice(spotOrderBook)
	futuresMidPrice := getMidPrice(futuresOrderBook)

	// the state is PositionOpening
	// check if the spot and futures positions are fully filled -> PositionReady
	if r.syncState.State == RoundOpening {
		spotRemaining := r.spotWorker.RemainingQuantity()
		futuresRemaining := r.futuresWorker.RemainingQuantity()
		spotIsDust := r.spotWorker.Market().IsDustQuantity(spotRemaining.Abs(), spotMidPrice)
		futuresIsDust := r.futuresWorker.Market().IsDustQuantity(futuresRemaining.Abs(), futuresMidPrice)

		if spotIsDust && futuresIsDust {
			r.syncState.State = RoundReady
			return
		}
	}

	// the state is PositionClosing
	// check if the spot and futures positions are fully closed -> PositionClosed
	if r.syncState.State == RoundClosing {
		spotFilled := r.spotWorker.FilledPosition()
		futuresFilled := r.futuresWorker.FilledPosition()
		spotIsDust := r.spotWorker.Market().IsDustQuantity(spotFilled.Abs(), spotMidPrice)
		futuresIsDust := r.futuresWorker.Market().IsDustQuantity(futuresFilled.Abs(), futuresMidPrice)

		if spotIsDust && futuresIsDust {
			r.syncState.State = RoundClosed
			r.logger.Infof("arbitrage round transit to closed state at %s: %s", currentTime.Format(time.RFC3339), r.spotWorker.Symbol())
		}
		return
	}
}

func (r *ArbitrageRound) CheckPositionDeviation(currentTime time.Time, maxDeviation fixedpoint.Value) (spotFilled, futuresFilled, deviation fixedpoint.Value) {
	// spot and futures position should be close to each other at all time.
	spotFilled = r.SpotWorker().FilledPosition()
	futuresFilled = r.FuturesWorker().FilledPosition()
	// calculate the deviation of the unhedged position
	spotFuturesRatio := fixedpoint.One
	// when closing, the spotFilled may reach zero
	if !spotFilled.IsZero() {
		spotFuturesRatio = futuresFilled.Div(spotFilled)
	}
	deviation = fixedpoint.One.Sub(spotFuturesRatio).Abs()
	deviationTooLarge := deviation.Compare(maxDeviation) > 0
	if deviationTooLarge {
		if r.syncState.LargeDeviationStartTime.IsZero() {
			r.syncState.LargeDeviationStartTime = currentTime
		}
	} else {
		r.syncState.LargeDeviationStartTime = time.Time{}
	}
	return spotFilled, futuresFilled, deviation
}

func (r *ArbitrageRound) DeviatedTooLong(currentTime time.Time, duration time.Duration) bool {
	if r.syncState.LargeDeviationStartTime.IsZero() {
		return false
	}
	return currentTime.Sub(r.syncState.LargeDeviationStartTime) > duration
}

func (r *ArbitrageRound) syncFuturesPosition(spotTrade types.Trade) {
	// sanity check
	if r.spotWorker.Symbol() != spotTrade.Symbol || spotTrade.IsFutures {
		return
	}
	// the filled spot position can be positive or negative
	filledSpotPosition := r.spotWorker.FilledPosition()
	// the futures target position should be always the negation of the spot filled position to maintain delta-neutral
	oriFuturesTargetPosition := r.futuresWorker.TargetPosition()
	futureTargetPosition := filledSpotPosition.Neg()
	r.logger.Infof("syncing futures position %s -> %s: %s",
		oriFuturesTargetPosition,
		futureTargetPosition,
		spotTrade,
	)
	r.futuresWorker.SetTargetPosition(futureTargetPosition)
}

func (r *ArbitrageRound) syncSpotPosition(futuresTrade types.Trade) {
	// sanity check
	if r.futuresWorker.Symbol() != futuresTrade.Symbol || !futuresTrade.IsFutures {
		return
	}

	// advance the spot worker's target to mirror the futures filled position.
	// This unblocks the spot leg to execute its offsetting slice on the next Tick.
	futuresFilled := r.futuresWorker.FilledPosition()
	spotTarget := futuresFilled.Neg()
	oriSpotTarget := r.spotWorker.TargetPosition()
	r.logger.Infof("syncing spot target on close %s -> %s: %s", oriSpotTarget, spotTarget, futuresTrade)
	r.spotWorker.SetTargetPosition(spotTarget)
}
