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
	Trade     types.Trade `json:"trade"`
	LastTried time.Time   `json:"lastTried"`
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
	futuresService FuturesService) *ArbitrageRound {
	var asset string
	if spotTwap.TargetPosition().Sign() > 0 {
		// long spot, short futures -> collateral is base asset
		asset = spotTwap.Market().BaseCurrency
	} else {
		// short spot, long futures -> collateral is quote asset
		asset = spotTwap.Market().QuoteCurrency
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
			Asset:               asset,
			State:               RoundPending,
			RetryTransfers:      make(map[uint64]*transferRetry),
			SyncedSpotTrades:    make(map[uint64]struct{}),
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
		r.syncState.ClosingTime.Add(r.syncState.ClosingDuration).Format(time.RFC3339),
	)
}

func (r *ArbitrageRound) SetSlackAlert(alert slackalert.SlackAlert) {
	r.slackAlert = alert
}

func (r *ArbitrageRound) NewNotification(isCritical bool) *roundNotification {
	return &roundNotification{
		ArbitrageRound: r,
		IsCritical:     isCritical,
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
				Value: n.syncState.ClosingTime.Add(n.syncState.ClosingDuration).Format(time.RFC3339),
				Short: true,
			})
	} else if n.HasStarted() {
		fields = append(fields,
			slack.AttachmentField{
				Title: "Start Time",
				Value: n.syncState.StartTime.Format(time.RFC3339),
				Short: true,
			},
			slack.AttachmentField{
				Title: "Min Holding Intervals",
				Value: fmt.Sprintf("%d", n.syncState.MinHoldingIntervals),
				Short: true,
			})
	}
	fields = append(fields,
		slack.AttachmentField{
			Title: "Funding Interval Start",
			Value: n.syncState.FundingIntervalStart.Format(time.RFC3339),
			Short: true,
		},
		slack.AttachmentField{
			Title: "Funding Interval End",
			Value: n.syncState.FundingIntervalEnd.Format(time.RFC3339),
			Short: true,
		})
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

	if r.syncState.State != RoundOpening {
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
		r.handleSpotTrade(transfer.Trade, currentTime)
	}
}

func (r *ArbitrageRound) SetRetryDuration(d time.Duration) {
	r.syncState.RetryDuration = d
}

func (r *ArbitrageRound) HandleSpotTrade(trade types.Trade, currentTime time.Time) {
	// lock the round to ensure the state is updated correctly when receiving trade updates from spot worker
	r.mu.Lock()
	defer r.mu.Unlock()

	r.handleSpotTrade(trade, currentTime)
}

func (r *ArbitrageRound) handleSpotTrade(trade types.Trade, currentTime time.Time) {
	if ok := r.spotWorker.Executor().AddTrade(trade); !ok {
		// the trade does not belong to the spot worker, skip
		return
	}
	r.logger.Infof("handling spot trade: %s", trade)
	// no matter the transfer succeeds or not, we should update the futures position so that we can stay in delta-neutral
	if _, found := r.syncState.SyncedSpotTrades[trade.ID]; !found {
		// the trade is not synced to futures yet
		r.syncFuturesPosition(trade)
	}
	r.syncState.SyncedSpotTrades[trade.ID] = struct{}{}

	// try to transfer asset from spot to futures as collateral.
	// if transfer fails, retry in the next tick until it succeeds
	timedCtx, cancel := context.WithTimeout(
		r.spotWorker.ctx,
		time.Second*20,
	)
	defer cancel()
	if err := r.futuresService.TransferFuturesAccountAsset(
		timedCtx, r.syncState.Asset, trade.Quantity, types.TransferIn,
	); err != nil {
		if transfer, found := r.syncState.RetryTransfers[trade.ID]; !found {
			bbgo.Notify("🚨 Round spot transfer %s %s failed (%s), retrying: %s",
				trade.Quantity.String(),
				r.syncState.Asset,
				currentTime.Format(time.RFC3339),
				err.Error(),
				trade,
			)
			r.syncState.RetryTransfers[trade.ID] = &transferRetry{
				Trade:     trade,
				LastTried: currentTime,
			}
		} else {
			transfer.LastTried = currentTime
		}
		return
	}
	// transfer succeeded, remove from retry list if exists
	delete(r.syncState.RetryTransfers, trade.ID)
	bbgo.Notify("➡️ Transfered %s %s from spot to futures",
		trade.Quantity.String(),
		r.syncState.Asset,
		trade,
	)
}

func (r *ArbitrageRound) HandleFuturesTrade(trade types.Trade, currentTime time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if ok := r.futuresWorker.Executor().AddTrade(trade); ok {
		r.logger.Infof("handling future trade: %s", trade)
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

func (r *ArbitrageRound) SetClosing(currentTime time.Time, duration time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// set spot target position to zero
	// the futures target position will be synced as the spot position is closing
	r.spotWorker.SetTargetPosition(fixedpoint.Zero)
	r.spotWorker.ResetTime(currentTime, duration)
	r.futuresWorker.ResetTime(currentTime, duration)

	r.syncState.State = RoundClosing
	r.syncState.ClosingTime = currentTime
	r.syncState.ClosingDuration = duration
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
		orderSide(remaining),
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
	r.spotWorker.Tick(currentTime, spotOrderBook)
	r.futuresWorker.Tick(currentTime, futuresOrderBook)

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

func (r *ArbitrageRound) syncFuturesPosition(spotTrade types.Trade) {
	// sanity check
	if r.spotWorker.Symbol() != spotTrade.Symbol {
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
