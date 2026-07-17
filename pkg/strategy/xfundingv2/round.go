package xfundingv2

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
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
	SetLeverage(ctx context.Context, symbol string, leverage int) error
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

	lastRebalanceTime time.Time
	rebalanceInterval time.Duration

	lastFundingIncomeSyncTime time.Time

	futuresService                                FuturesService
	spotExchangeFeeRates, futuresExchangeFeeRates map[types.ExchangeName]types.ExchangeFee

	spotSession, futuresSession *bbgo.ExchangeSession
	retryTransferTickC          chan time.Time

	logger     logrus.FieldLogger
	slackAlert slackalert.SlackAlert
}

func NewArbitrageRound(
	fundingRate *types.PremiumIndex,
	spotExchangeName, futuresExchangeName types.ExchangeName,
	minHoldingIntervals, fundingIntervalHours int,
	leverage fixedpoint.Value,
	spotTwap, futuresTwap *TWAPWorker,
	futuresService FuturesService,
	direction types.PositionType,
	rebalanceInterval time.Duration,
) *ArbitrageRound {
	policy, err := newDirectionPolicy(direction, spotTwap.Market())
	if err != nil {
		panic(fmt.Sprintf("NewArbitrageRound: %v", err))
	}
	fundingIntervalStart := fundingRate.NextFundingTime.Add(-time.Duration(fundingIntervalHours) * time.Hour)
	fundingIntervalEnd := fundingRate.NextFundingTime.Add(-time.Second)
	return &ArbitrageRound{
		syncState: ArbitrageRoundSyncState{
			ID:                          uuid.NewString(),
			Symbol:                      spotTwap.Symbol(),
			TriggeredFundingRate:        fundingRate.LastFundingRate,
			TriggeredSpotTargetPosition: spotTwap.TargetPosition(),
			MinHoldingIntervals:         minHoldingIntervals,
			FundingIntervalHours:        fundingIntervalHours,
			Leverage:                    leverage,
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
		rebalanceInterval:  rebalanceInterval,
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

func (r *ArbitrageRound) StartedAt() time.Time {
	return r.syncState.StartAt
}

func (r *ArbitrageRound) ClosedAt() time.Time {
	return r.syncState.ClosedAt
}

func (r *ArbitrageRound) ReadyAt() time.Time {
	return r.syncState.ReadyAt
}

func (r *ArbitrageRound) ClosingAt() time.Time {
	return r.syncState.ClosingAt
}

func (r *ArbitrageRound) HasStarted() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.hasStarted()
}

func (r *ArbitrageRound) hasStarted() bool {
	return !r.syncState.StartAt.IsZero()
}

func (r *ArbitrageRound) TriggeredFundingRate() fixedpoint.Value {
	return r.syncState.TriggeredFundingRate
}

func (r *ArbitrageRound) TriggeredTargetPosition() fixedpoint.Value {
	return r.syncState.TriggeredSpotTargetPosition
}

func (r *ArbitrageRound) NumHoldingIntervals(currentTime time.Time) int {
	if r.syncState.StartAt.IsZero() {
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
			r.syncState.StartAt.Format(time.RFC3339),
		)
	}
	return fmt.Sprintf(
		"ArbitrageRound(symbol=%s, state=%s, spot=%s, futures=%s, closingTime=%s, expectedCloseTime=%s)",
		r.spotWorker.Symbol(),
		r.syncState.State,
		r.spotWorker.FilledPosition(),
		r.futuresWorker.FilledPosition(),
		r.syncState.ClosingAt.Format(time.RFC3339),
		r.syncState.ClosingAt.Add(r.syncState.ClosingDuration.Duration()).Format(time.RFC3339),
	)
}

func (r *ArbitrageRound) SetSlackAlert(alert slackalert.SlackAlert) {
	r.slackAlert = alert
}

func (r *ArbitrageRound) NewCriticalNotification(spotOrderBook, futuresOrderBook types.OrderBook) *roundNotification {
	return &roundNotification{
		ArbitrageRound: r,
		IsCritical:     true,

		spotOrderBook:    spotOrderBook,
		futuresOrderBook: futuresOrderBook,
	}
}

func (r *ArbitrageRound) NewNotification(spotOrderBook, futuresOrderBook types.OrderBook) *roundNotification {
	return &roundNotification{
		ArbitrageRound: r,
		IsCritical:     false,

		spotOrderBook:    spotOrderBook,
		futuresOrderBook: futuresOrderBook,
	}
}

type roundNotification struct {
	*ArbitrageRound
	IsCritical bool

	spotOrderBook, futuresOrderBook types.OrderBook
}

func (n *roundNotification) SlackAttachment() slack.Attachment {
	title := fmt.Sprintf("Arbitrage Round %s (%s)", n.SpotSymbol(), n.syncState.State)
	fields := []slack.AttachmentField{
		{
			Title: "Round ID",
			Value: n.syncState.ID,
			Short: false,
		},
		{
			Title: "Triggered Spot Position",
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
		{
			Title: "Start Time",
			Value: n.syncState.StartAt.Format(time.RFC3339),
			Short: true,
		},
	}
	switch n.State() {
	case RoundClosing:
		fields = append(fields,
			slack.AttachmentField{
				Title: "Closing Time",
				Value: n.syncState.ClosingAt.Format(time.RFC3339),
				Short: true,
			},
			slack.AttachmentField{
				Title: "Expected Close Time",
				Value: n.syncState.ClosingAt.Add(n.syncState.ClosingDuration.Duration()).Format(time.RFC3339),
				Short: false,
			})
	case RoundOpening, RoundReady:
		minHoldingHours := n.syncState.MinHoldingIntervals * n.syncState.FundingIntervalHours
		expectedClosingTime := n.syncState.FundingIntervalStart.Add(time.Duration(minHoldingHours) * time.Hour)
		fields = append(fields,
			slack.AttachmentField{
				Title: "Expected Closing Time",
				Value: expectedClosingTime.Format(time.RFC3339),
				Short: true,
			},
			slack.AttachmentField{
				Title: "Funding Interval Start",
				Value: n.syncState.FundingIntervalStart.Format(time.RFC3339),
				Short: true,
			},
			slack.AttachmentField{
				Title: "Min Holding Intervals",
				Value: fmt.Sprintf("%d", n.syncState.MinHoldingIntervals),
				Short: true,
			},
		)
	case RoundClosed:
		fields = append(fields,
			slack.AttachmentField{
				Title: "Closed Time",
				Value: n.syncState.ClosedAt.Format(time.RFC3339),
				Short: true,
			},
		)
	}
	var spotAvgCost, futuresAvgCost fixedpoint.Value
	if n.hasStarted() {
		if n.State() == RoundClosed {
			realizedPnL := n.RealizedPnL()
			spotAvgCost = realizedPnL.SpotPosition.AverageCost
			futuresAvgCost = realizedPnL.FuturesPosition.AverageCost
			fields = append(fields, realizedPnLFields(realizedPnL)...)
		} else {
			unrealizedPnL := n.UnrealizedPnL(
				n.spotOrderBook,
				n.futuresOrderBook,
			)
			spotAvgCost = unrealizedPnL.SpotPosition.AverageCost
			futuresAvgCost = unrealizedPnL.FuturesPosition.AverageCost
			fields = append(fields, unrealizedPnLFields(unrealizedPnL)...)
		}
	}
	spotActiveOrder := n.spotWorker.activeOrder
	spotOrderField := slack.AttachmentField{
		Title: "Spot Active Order",
		Value: "NONE",
		Short: true,
	}
	if spotActiveOrder != nil {
		spotOrderField.Value = fmt.Sprintf(
			"%s %s/%s@%s",
			spotActiveOrder.Side,
			spotActiveOrder.ExecutedQuantity,
			spotActiveOrder.Quantity,
			spotActiveOrder.Price,
		)
	}
	futuresActiveOrder := n.futuresWorker.activeOrder
	futuresOrderField := slack.AttachmentField{
		Title: "Futures Active Order",
		Value: "NONE",
		Short: true,
	}
	if futuresActiveOrder != nil {
		futuresOrderField.Value = fmt.Sprintf(
			"%s %s/%s@%s",
			futuresActiveOrder.Side,
			futuresActiveOrder.ExecutedQuantity,
			futuresActiveOrder.Quantity,
			futuresActiveOrder.Price,
		)
	}

	fields = append(fields,
		slack.AttachmentField{
			Title: "Spot Filled Position",
			Value: fmt.Sprintf("%s@%s", n.spotWorker.FilledPosition().String(), spotAvgCost.String()),
			Short: true,
		},
		slack.AttachmentField{
			Title: "Futures Filled Position",
			Value: fmt.Sprintf("%s@%s", n.futuresWorker.FilledPosition().String(), futuresAvgCost.String()),
			Short: true,
		},
		spotOrderField,
		futuresOrderField,
	)

	text := "Arbitrage Round Details"
	if n.IsCritical && len(n.slackAlert.Mentions) > 0 {
		text += " cc " + strings.Join(n.slackAlert.Mentions, " ")
	}
	return slack.Attachment{
		Title:  title,
		Text:   text,
		Color:  n.stateColor(),
		Fields: fields,
	}
}

func realizedPnLFields(realizedPnL *RoundRealizedPnL) []slack.AttachmentField {
	return []slack.AttachmentField{
		{
			Title: "Spot PnL",
			Value: fmt.Sprintf(
				"%s (Net %s)",
				realizedPnL.SpotProfitStats.AccumulatedPnL.String(),
				realizedPnL.SpotProfitStats.AccumulatedNetProfit.String(),
			),
			Short: true,
		},
		{
			Title: "Futures PnL",
			Value: fmt.Sprintf("%s (Net %s)",
				realizedPnL.FuturesProfitStats.AccumulatedPnL.String(),
				realizedPnL.FuturesProfitStats.AccumulatedNetProfit.String()),
			Short: true,
		},
		{
			Title: "Funding Income",
			Value: realizedPnL.FundingIncome.String(),
			Short: true,
		},
		{
			Title: "Total PnL",
			Value: realizedPnL.TotalPnL().String(),
			Short: true,
		},
	}
}

func unrealizedPnLFields(unrealizedPnL *RoundUnrealizedPnL) []slack.AttachmentField {
	return []slack.AttachmentField{
		{
			Title: "Unrealized Spot PnL",
			Value: unrealizedPnL.UnrealizedSpotPnL.String(),
			Short: true,
		},
		{
			Title: "Realized Spot Net PnL",
			Value: unrealizedPnL.SpotProfitStats.AccumulatedNetProfit.String(),
			Short: true,
		},
		{
			Title: "Unrealized Futures PnL",
			Value: unrealizedPnL.UnrealizedFuturesPnL.String(),
			Short: true,
		},
		{
			Title: "Realized Futures Net PnL",
			Value: unrealizedPnL.FuturesProfitStats.AccumulatedNetProfit.String(),
			Short: true,
		},
		{
			Title: "Total Funding Income",
			Value: unrealizedPnL.FundingIncome.String(),
			Short: true,
		},
		{
			Title: "Total PnL",
			Value: unrealizedPnL.TotalPnL().String(),
			Short: true,
		},
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

	if r.syncState.StartAt.IsZero() {
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
	if r.syncState.StartAt.IsZero() || r.syncState.StartAt.After(currentTime) {
		return fmt.Errorf("query time is before the round start time: current %s v.s start %s",
			currentTime.Format(time.RFC3339), r.syncState.StartAt.Format(time.RFC3339),
		)
	}

	syncDuration := time.Duration(r.syncState.FundingIntervalHours/2) * time.Hour
	if !r.lastFundingIncomeSyncTime.IsZero() && currentTime.Sub(r.lastFundingIncomeSyncTime) <= syncDuration {
		return nil
	}
	r.lastFundingIncomeSyncTime = currentTime
	q := batch.BinanceFuturesIncomeBatchQuery{
		BinanceFuturesIncomeHistoryService: r.futuresService,
	}
	symbol := r.futuresWorker.Symbol()
	dataC, errC := q.Query(ctx, symbol, binanceapi.FuturesIncomeFundingFee, r.syncState.StartAt, currentTime)
	shouldContinue := true
	for shouldContinue {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case income, ok := <-dataC:
			if !ok {
				shouldContinue = false
				continue
			}
			switch income.IncomeType {
			case binanceapi.FuturesIncomeFundingFee:
				record := FundingFee{
					RoundID: r.syncState.ID,
					Asset:   income.Asset,
					Amount:  income.Income,
					Txn:     income.TranId,
					Time:    income.Time.Time(),
				}
				r.syncState.FundingFeeRecords[income.TranId] = record
			}
		case err, ok := <-errC:
			if !ok {
				shouldContinue = false
				continue
			}

			return err

		}
	}
	r.logger.Debugf("synced funding fee records: %+v", r.syncState.FundingFeeRecords)
	return nil
}

func (r *ArbitrageRound) Start(ctx context.Context,
	spotSession, futuresSession *bbgo.ExchangeSession,
	currentTime time.Time,
) error {
	if r.syncState.StartAt.IsZero() {
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

		r.spotSession = spotSession
		r.futuresSession = futuresSession

		go r.retryTransferWorker(ctx, r.retryTransferTickC)

		r.syncState.StartAt = currentTime
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

	if !r.hasStarted() {
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
			r.handleFuturesTradeForClose(transfer.Trade, r.futuresSession.Account, currentTime)
		case types.TransferIn:
			// the round should be in opening state
			r.handleSpotTradeForOpen(transfer.Trade, r.spotSession.Account, currentTime)
		default:
			r.logger.Warnf("unknown transfer direction for retry: %s", transfer.Direction)
		}
	}
}

func (r *ArbitrageRound) SetRetryDuration(d time.Duration) {
	r.syncState.RetryDuration = d
}

// HandleSpotTrade handles a spot trade, including update filled position, transfer collateral and sync futures position if the round is opening.
func (r *ArbitrageRound) HandleSpotTrade(trade types.Trade, spotAccount *types.Account, currentTime time.Time) {
	// lock the round to ensure the state is updated correctly when receiving trade updates from spot worker
	r.mu.Lock()
	defer r.mu.Unlock()

	if ok := r.spotWorker.Executor().AddTrade(trade); !ok {
		// the trade does not belong to the spot worker, skip
		return
	}
	r.logger.Debugf("round spot trade: %s", trade)

	if r.syncState.State == RoundOpening {
		r.logger.Infof("handling spot trade (open): %s", trade)
		r.handleSpotTradeForOpen(trade, spotAccount, currentTime)
	} else {
		// the round is ready or closing, just log the spot trade
		r.logger.Infof("received spot trade: %s", trade)
	}
}

// handleSpotTradeForOpen is the mirror of handleFuturesTradeForClose: during opening, spot is the leader.
// After each spot fill, the futures position is synced to keep delta-neutral and the collateral asset is
// transferred to futures so the futures leg can use it as collateral for its offsetting trade.
func (r *ArbitrageRound) handleSpotTradeForOpen(trade types.Trade, spotAccount *types.Account, currentTime time.Time) {
	// move the collateral asset received from the spot fill onto the futures
	// account so the futures leg can use it as margin.
	transferAmount := r.syncState.DirectionPolicy.TransferAmountFromSpotTrade(trade)

	timedCtx, cancel := context.WithTimeout(
		r.spotWorker.ctx,
		time.Second*20,
	)
	defer cancel()
	asset := r.syncState.DirectionPolicy.CollateralAsset()
	if trade.FeeCurrency == asset {
		transferAmount = transferAmount.Sub(trade.Fee)
	}
	available := spotAccount.Balances()[asset].Available
	if !available.IsZero() {
		transferAmount = fixedpoint.Min(transferAmount, available)
	}

	if transferAmount.Sign() <= 0 {
		// nothing left to transfer (may have been transferred by rebalancing)
		r.logger.Warnf("no collateral asset available to transfer to futures: %s (available: %s %s)", trade.Symbol, available, asset)
		// consider the transfer succeeded -> delete the retry task and sync the futures position
		delete(r.syncState.RetryTransfers, trade.ID)
		if _, found := r.syncState.SyncedSpotTrades[trade.ID]; !found {
			r.syncFuturesPosition(trade)
			r.syncState.SyncedSpotTrades[trade.ID] = struct{}{}
		}
		return
	}

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
	r.syncState.TransferInAmount = r.syncState.TransferInAmount.Add(transferAmount)
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
func (r *ArbitrageRound) HandleFuturesTrade(trade types.Trade, futuresAccount *types.Account, currentTime time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if ok := r.futuresWorker.Executor().AddTrade(trade); !ok {
		// the trade does not belong to the futures worker, skip
		return
	}

	if r.syncState.State == RoundClosing {
		r.logger.Infof("handling future trade (closing): %s", trade)
		r.handleFuturesTradeForClose(trade, futuresAccount, currentTime)
	} else {
		// the round is opening or ready, just log the futures trade
		r.logger.Infof("received futures trade: %s", trade)
	}
}

func (r *ArbitrageRound) HandleSpotOrderUpdate(update types.Order) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.hasOrder(update.OrderID) {
		return
	}

	handleOrderUpdate(r.spotWorker, update)
}

func (r *ArbitrageRound) HandleFuturesOrderUpdate(update types.Order) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.hasOrder(update.OrderID) {
		return
	}

	handleOrderUpdate(r.futuresWorker, update)
}

func handleOrderUpdate(twapWorker *TWAPWorker, update types.Order) {
	activeOrder := twapWorker.ActiveOrder()
	if activeOrder != nil && activeOrder.OrderID == update.OrderID {
		activeOrder.Update(update)
	}
	twapWorker.Executor().UpdateOrder(update)
}

// handleFuturesTradeForClose is the mirror of handleSpotTradeForOpen: during
// closing, futures is the leader. After each futures fill reduces the position,
// the freed collateral asset is transferred back to spot and the spot worker's
// target is advanced so it can execute its offsetting trade with that asset.
func (r *ArbitrageRound) handleFuturesTradeForClose(trade types.Trade, futuresAccount *types.Account, currentTime time.Time) {
	if !bbgo.IsBackTesting {
		// simple hack: wait for 5 seconds to let the futures account update its balance if it's not a backtesting environment.
		// this would make the available withdraw amount to reflect the trade fill
		time.Sleep(5 * time.Second)
	}
	// transfer the freed collateral asset back to spot so the spot leg can use it to close its position.
	transferAmount := r.syncState.DirectionPolicy.TransferAmountFromFuturesTrade(trade)
	asset := r.syncState.DirectionPolicy.CollateralAsset()

	if asset == trade.FeeCurrency {
		transferAmount = transferAmount.Sub(trade.Fee)
	}
	// read the balance after the delay above so the max withdraw amount reflects the trade fill.
	if w := futuresAccount.Balances()[asset].MaxWithdrawAmount; w != nil {
		r.logger.Debugf("max withdraw amount on futures account: %s %s", w.String(), asset)
		transferAmount = fixedpoint.Min(transferAmount, *w)
	}

	if transferAmount.Sign() <= 0 {
		// nothing left to transfer
		// remove from retry list if exists
		r.logger.Warnf("no collateral asset available to transfer back to spot: %s", asset)
		delete(r.syncState.RetryTransfers, trade.ID)
		// consider the transfer succeeded and sync the spot position
		if _, found := r.syncState.SyncedFuturesTrades[trade.ID]; !found {
			r.syncSpotPosition(trade)
			r.syncState.SyncedFuturesTrades[trade.ID] = struct{}{}
		}
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
	r.syncState.TransferOutAmount = r.syncState.TransferOutAmount.Add(transferAmount)
	delete(r.syncState.RetryTransfers, trade.ID)
	bbgo.Notify("⬅️ Transfered %s %s from futures to spot",
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

// Prepare prepares the round to be ready for resume, such as doing neceessary transfers and syncing the positions.
func (r *ArbitrageRound) Prepare(
	ctx context.Context,
	spotSession, futuresSession *bbgo.ExchangeSession,
) error {
	timedCtx, cancel := context.WithTimeout(
		ctx,
		time.Second*20,
	)
	defer cancel()

	switch r.syncState.State {
	case RoundOpening:
		return r.prepareOpening(timedCtx, spotSession)
	case RoundClosing:
		return r.prepareClosing(timedCtx, futuresSession)
	}

	return nil
}

func (r *ArbitrageRound) prepareOpening(
	ctx context.Context,
	spotSession *bbgo.ExchangeSession,
) error {
	now := time.Now()
	var expectedTransferIn fixedpoint.Value
	trades := r.spotWorker.Executor().AllTrades()
	var spotSide types.SideType
	for idx, trade := range trades {
		if idx == 0 {
			spotSide = trade.Side
		} else if spotSide != trade.Side {
			return fmt.Errorf("all spot trades should have the same side when opening: %s vs %s", spotSide, trade.Side)
		}
		amount := r.syncState.DirectionPolicy.TransferAmountFromSpotTrade(trade)
		expectedTransferIn = expectedTransferIn.Add(amount)
	}
	transferDiff := expectedTransferIn.Sub(r.syncState.TransferInAmount)
	asset := r.syncState.DirectionPolicy.CollateralAsset()
	balance := spotSession.GetAccount().Balances()[asset]
	if transferDiff.Sign() > 0 && balance.Available.Compare(transferDiff) > 0 {
		r.logger.Infof("expected transfer in amount: %s, actual transfer in amount: %s, diff: %s, available: %s",
			expectedTransferIn,
			r.syncState.TransferInAmount,
			transferDiff,
			balance.Available,
		)
		if err := r.futuresService.TransferFuturesAccountAsset(ctx, asset, transferDiff, types.TransferIn); err != nil {
			return fmt.Errorf("failed to transfer in %s %s: %w", transferDiff.String(), asset, err)
		}
		r.syncState.TransferInAmount = r.syncState.TransferInAmount.Add(transferDiff)
	}
	// transfer succeeded, need to sync the futures position to keep delta-neutral
	targetPosition := r.spotWorker.FilledPosition().Neg()
	r.futuresWorker.SetTargetPosition(targetPosition)
	// don't place order but reset the time to let the TWAP worker to sync the position
	// For example, considering the case where:
	// 1. the futures worker's active order is not filled but the target position is high
	// 2. the active order is canceled when the strategy shut down
	// 3. the round is restore and remains in opening state when the strategy is restarted
	// If we place order directly, we may place a large order to sync the position, which may cause a large slippage.
	r.spotWorker.ResetTime(now, r.spotWorker.Duration())
	r.futuresWorker.ResetTime(now, r.futuresWorker.Duration())
	r.Resume()
	r.syncState.LargeDeviationStartTime = time.Time{}

	return nil
}

func (r *ArbitrageRound) prepareClosing(
	ctx context.Context,
	futuresSession *bbgo.ExchangeSession,
) error {
	now := time.Now()
	var expectedTransferOut fixedpoint.Value
	trades := r.futuresWorker.Executor().AllTrades()
	// if we are short futures -> closing trades are buy trades
	// if we are long futures -> closing trades are sell trades
	var closingTradeSide types.SideType
	shortFutures := r.syncState.TriggeredSpotTargetPosition.Sign() > 0
	if shortFutures {
		closingTradeSide = types.SideTypeBuy
	} else {
		closingTradeSide = types.SideTypeSell
	}
	for _, trade := range trades {
		if trade.Side != closingTradeSide {
			continue
		}
		amount := r.syncState.DirectionPolicy.TransferAmountFromFuturesTrade(trade)
		expectedTransferOut = expectedTransferOut.Add(amount)
	}
	transferDiff := expectedTransferOut.Sub(r.syncState.TransferOutAmount)
	asset := r.syncState.DirectionPolicy.CollateralAsset()
	balance := futuresSession.GetAccount().Balances()[asset]
	maxWithdraw := balance.MaxWithdrawAmount
	if maxWithdraw != nil {
		transferDiff = fixedpoint.Min(transferDiff, *maxWithdraw)
	}
	if transferDiff.Sign() > 0 {
		r.logger.Infof("expected transfer out amount: %s, actual transfer out amount: %s, diff: %s, max withdraw: %s",
			expectedTransferOut,
			r.syncState.TransferOutAmount,
			transferDiff,
			maxWithdraw,
		)
		if err := r.futuresService.TransferFuturesAccountAsset(ctx, asset, transferDiff, types.TransferOut); err != nil {
			return fmt.Errorf("failed to transfer out %s %s: %w", transferDiff.String(), asset, err)
		}
		r.syncState.TransferOutAmount = r.syncState.TransferOutAmount.Add(transferDiff)
	}
	// transfer succeeded, need to sync the spot position to keep delta-neutral
	targetPosition := r.futuresWorker.FilledPosition().Neg()
	r.spotWorker.SetTargetPosition(targetPosition)
	// don't place order but reset the time to let the TWAP worker to sync the position
	// For example, considering the case where:
	// 1. the spot worker's active order is not filled and the target position is high.
	// 2. the active order is canceled when the strategy shut down
	// 3. the round is set to closing when the strategy is restarted
	// If we place order directly, we may place a large order to sync the position, which may cause a large slippage.
	r.spotWorker.ResetTime(now, r.spotWorker.Duration())
	r.futuresWorker.ResetTime(now, r.futuresWorker.Duration())
	r.Resume()
	r.syncState.LargeDeviationStartTime = time.Time{}

	return nil
}

func (r *ArbitrageRound) SetLogger(logger logrus.FieldLogger) {
	r.logger = logger
}

func (r *ArbitrageRound) ID() string {
	return r.syncState.ID
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

	// While closing, futures is the leader: drive its target to zero so it
	// starts reducing the position. The spot target stays where it is and is
	// re-synced after each futures fill in handleFuturesTradeForClose, so spot
	// only trades once the collateral has been transferred back from futures.
	r.futuresWorker.SetTargetPosition(fixedpoint.Zero)
	r.spotWorker.ResetTime(currentTime, duration)
	r.futuresWorker.ResetTime(currentTime, duration)

	r.syncState.State = RoundClosing
	r.syncState.ClosingAt = currentTime
	r.syncState.ClosingDuration = duration
	// empty the retry transfer tasks to avoid retrying old opening trades when closing
	r.syncState.RetryTransfers = make(map[uint64]*transferRetry)
}

// setReady updates the round state to ready without locking. The caller must
// already hold r.mu.
func (r *ArbitrageRound) setReady(currentTime time.Time) {
	if !r.syncState.ReadyAt.IsZero() {
		return
	}

	if oriOrder := r.spotWorker.syncAndResetActiveOrder(); oriOrder != nil {
		r.logger.Debugf("[setReady] reseting spot order: %s", oriOrder)
	}
	if oriOrder := r.futuresWorker.syncAndResetActiveOrder(); oriOrder != nil {
		r.logger.Debugf("[setReady] reseting futures order: %s", oriOrder)
	}
	r.syncState.State = RoundReady
	r.syncState.ReadyAt = currentTime
}

// setClosed updates the round state to closed without locking. The caller must
// already hold r.mu.
func (r *ArbitrageRound) setClosed(currentTime time.Time) {
	if !r.syncState.ClosedAt.IsZero() {
		return
	}
	r.syncState.State = RoundClosed
	r.syncState.ClosedAt = currentTime
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
	r.logger.Debugf("[Cleanup] remaining futures quantity: %s", remaining)
	if remaining.IsZero() {
		return nil
	}

	r.logger.Infof("[Cleanup] closing remaining futures position: %s", remaining)
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
	if err != nil {
		return fmt.Errorf("[CloseFuturesPosition] failed to query position risk after closing order filled: %w", err)
	}
	if len(risks) == 0 {
		// no position
		return nil
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
func (r *ArbitrageRound) Tick(ctx context.Context, currentTime time.Time, spotOrderBook types.OrderBook, futuresOrderBook types.OrderBook) {
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

	if err := r.rebalance(ctx, currentTime, futuresOrderBook); err != nil {
		r.logger.WithError(err).Errorf("failed to rebalance round: %s", r.String())
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
			r.setReady(currentTime)
			return
		}
	}

	// the state is PositionClosing
	// check if the spot and futures positions are fully closed -> PositionClosed
	if r.syncState.State == RoundClosing {
		spotFilled := r.spotWorker.FilledPosition()
		futuresFilled := r.futuresWorker.FilledPosition()
		r.logger.Debugf("[Tick] spot filled: %s, futures filled: %s", spotFilled, futuresFilled)
		spotIsDust := r.spotWorker.Market().IsDustQuantity(spotFilled.Abs(), spotMidPrice)
		futuresIsDust := r.futuresWorker.Market().IsDustQuantity(futuresFilled.Abs(), futuresMidPrice)

		if spotIsDust && futuresIsDust {
			r.setClosed(currentTime)
			r.logger.Infof("arbitrage round transit to closed state at %s: %s", currentTime.Format(time.RFC3339), r.spotWorker.Symbol())
		}
		return
	}
}

type PositionDeviation struct {
	SpotFilled       fixedpoint.Value
	FuturesFilled    fixedpoint.Value
	DeviatedQuantity fixedpoint.Value
	DeviateTooLong   bool
}

func (r *ArbitrageRound) CheckPositionDeviation(currentTime time.Time, maxMoqDeviation fixedpoint.Value, duration time.Duration) PositionDeviation {
	// MOQ: minimum order quantity
	spotMOQ := r.spotWorker.Market().MinQuantity
	futuresMOQ := r.futuresWorker.Market().MinQuantity
	thresMOQ := fixedpoint.Max(spotMOQ, futuresMOQ)
	// calculate the deviation of the unhedged position
	thresholdQuantity := thresMOQ.Mul(maxMoqDeviation)

	// spot and futures position should be close to each other at all time.
	spotFilled := r.SpotWorker().FilledPosition()
	futuresFilled := r.FuturesWorker().FilledPosition()
	deviation := spotFilled.Add(futuresFilled).Abs()
	deviationTooLarge := deviation.Compare(thresholdQuantity) >= 0
	if deviationTooLarge {
		if r.syncState.LargeDeviationStartTime.IsZero() {
			r.syncState.LargeDeviationStartTime = currentTime
		}
	} else {
		r.syncState.LargeDeviationStartTime = time.Time{}
	}
	deviateTooLong := false
	if !r.syncState.LargeDeviationStartTime.IsZero() && currentTime.Sub(r.syncState.LargeDeviationStartTime) > duration {
		deviateTooLong = true
	}
	return PositionDeviation{
		SpotFilled:       spotFilled,
		FuturesFilled:    futuresFilled,
		DeviatedQuantity: deviation,
		DeviateTooLong:   deviateTooLong,
	}
}

func (r *ArbitrageRound) syncFuturesPosition(spotTrade types.Trade) {
	// sanity check
	if _, ok := r.spotWorker.Executor().GetOrder(spotTrade.OrderID); !ok {
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
	if _, ok := r.futuresWorker.Executor().GetOrder(futuresTrade.OrderID); !ok {
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

func (r *ArbitrageRound) rebalance(ctx context.Context, currentTime time.Time, futuresOrderBook types.OrderBook) error {
	if !r.lastRebalanceTime.IsZero() && currentTime.Sub(r.lastRebalanceTime) < r.rebalanceInterval {
		return nil
	}
	r.lastRebalanceTime = currentTime
	switch r.syncState.State {
	case RoundOpening:
		return r.rebalanceOpening(ctx, futuresOrderBook)
	case RoundClosing:
		return r.rebalanceClosing(ctx)
	}
	return nil
}

func (r *ArbitrageRound) rebalanceOpening(ctx context.Context, futuresOrderBook types.OrderBook) error {
	timedCtx, cancel := context.WithTimeout(ctx, time.Second*20)
	defer cancel()

	shortFutures := r.syncState.TriggeredSpotTargetPosition.Sign() > 0
	if shortFutures {
		// rebalance the short futures leg when opening
		// check if there is any remaining quantity on the spot account to be transferred to futures account
		spotAccount, err := r.spotSession.UpdateAccount(timedCtx)
		if err != nil {
			return fmt.Errorf("failed to update spot account: %w", err)
		}
		baseAsset := r.CollateralAsset()
		baseAvailable := spotAccount.Balances()[baseAsset].Available
		if baseAvailable.Sign() > 0 {
			// transfer the available collateral asset from spot to futures
			if err := r.futuresService.TransferFuturesAccountAsset(timedCtx, baseAsset, baseAvailable, types.TransferIn); err != nil {
				return fmt.Errorf("failed to transfer %s %s from spot to futures: %w", baseAvailable.String(), baseAsset, err)
			}
			r.syncState.TransferInAmount = r.syncState.TransferInAmount.Add(baseAvailable)
			bbgo.Notify("➡️ Transfered %s %s from spot to futures to rebalance",
				baseAvailable.String(),
				baseAsset,
			)
		}
		// check if there is sufficient margin on the futures account to open the position, if not, transfer from spot account
		futuresRemaining := r.futuresWorker.RemainingQuantity().Abs()
		if activeOrder := r.futuresWorker.ActiveOrder(); activeOrder != nil {
			// deduct the active order remaining quantity from the futures remaining quantity
			futuresRemaining = futuresRemaining.Sub(activeOrder.GetRemainingQuantity())
		}
		if futuresRemaining.Sign() <= 0 {
			r.logger.Debugf("non-positive remaining quantity for futures, no need for rebalance: %s", r)
			return nil
		}
		// short futures -> sell order on futures, use the best bid price
		bestBid, ok := futuresOrderBook.BestBid()
		if !ok {
			r.logger.Debugf("cannot get the best bid, skipping rebalance: %s", r)
			return nil
		}
		price := bestBid.Price
		futuresAccount, err := r.futuresSession.UpdateAccount(timedCtx)
		if err != nil {
			return fmt.Errorf("failed to update futures account: %w", err)
		}
		requiredMargin := price.Mul(futuresRemaining).Div(r.syncState.Leverage)
		futuresAvailable := futuresAccount.FuturesInfo.AvailableBalance
		if futuresAvailable.Compare(requiredMargin) < 0 {
			r.logger.Warnf("detected insufficient available balance on futures account to open %s position: required %s, available %s",
				r.FuturesSymbol(), requiredMargin, futuresAvailable,
			)
			// add 10% buffer
			transferAmount := requiredMargin.Sub(futuresAvailable).Mul(fixedpoint.NewFromFloat(1.1))
			market := r.spotWorker.Market()
			spotAvailable := spotAccount.Balances()[market.QuoteCurrency].Available
			if spotAvailable.Compare(transferAmount) < 0 {
				return fmt.Errorf("insufficient %s balance on spot account (%s, need %s) to rebalance round: %s",
					market.QuoteCurrency, spotAvailable, transferAmount, r.String())
			}
			// transfer quote asset from spot to futures
			if err := r.futuresService.TransferFuturesAccountAsset(timedCtx, market.QuoteCurrency, transferAmount, types.TransferIn); err != nil {
				return fmt.Errorf("failed to transfer %s %s from spot to futures: %w", transferAmount.String(), market.QuoteCurrency, err)
			}
			r.logger.Infof("rebalance: transferred %s %s from spot to futures to open position: %s",
				requiredMargin.String(), market.QuoteCurrency, r.String())
		}
	} else {
		// TODO: rebalance the long futures leg when opening
	}
	return nil
}

func (r *ArbitrageRound) rebalanceClosing(ctx context.Context) error {
	timedCtx, cancel := context.WithTimeout(ctx, time.Second*20)
	defer cancel()

	r.logger.Debugf("rebalance closing round: %s", r.SpotSymbol())

	shortFutures := r.syncState.TriggeredSpotTargetPosition.Sign() > 0
	if shortFutures {
		spotAccount, err := r.spotSession.UpdateAccount(timedCtx)
		if err != nil {
			return fmt.Errorf("failed to update spot account: %w", err)
		}
		futuresAccount, err := r.futuresSession.UpdateAccount(timedCtx)
		if err != nil {
			return fmt.Errorf("failed to update futures account: %w", err)
		}
		// rebalance the short futures leg for negative unrealized PnL
		// negative unrealized PnL will lock available collateral on futures account and prevent us to transfer out the asset back to spot account.
		// Overtime, it may cause an unexpected position risk on overall positions, both spot and futures.
		// We check the net balance of the quote asset on futures account and transfer from spot account to futures account if it's negative.
		baseAsset := r.CollateralAsset()
		futuresMarket := r.futuresWorker.Market()
		quoteAsset := futuresMarket.QuoteCurrency
		r.logger.Debugf("base asset: %s, quote asset: %s", baseAsset, quoteAsset)
		if b, ok := futuresAccount.Balance(quoteAsset); ok && b.Net().Sign() < 0 {
			// rebalance the short futures leg for negative net quote asset balance
			spotMarket := r.spotWorker.Market()
			// transfer 105% of the unrealized PnL from spot to futures to cover the loss
			transferAmount := b.Net().Abs().Mul(fixedpoint.NewFromFloat(1.05))
			transferAmount = spotMarket.TruncateQuantity(transferAmount)
			spotBalance := spotAccount.Balances()[quoteAsset]
			spotAvailable := spotBalance.Available
			if transferAmount.Compare(spotAvailable) > 0 {
				return fmt.Errorf("insufficient %s balance on spot account (%s) to rebalance round: %s", quoteAsset, spotAvailable, r.String())
			}
			// transfer quote asset from spot to futures
			if err := r.futuresService.TransferFuturesAccountAsset(timedCtx, quoteAsset, transferAmount, types.TransferIn); err != nil {
				return fmt.Errorf("failed to transfer %s %s from spot to futures: %w", transferAmount.String(), quoteAsset, err)
			}
		}
		// 2. transfer the base asset from futures to spot
		// check the accumulated transfer out amount and the expected one
		// if the expected transfer amount is larger than the accumulated one, transfer the difference back to spot
		// NOTE: need to check the max withdraw
		// update futures account info for the latest max withdraw amount
		accTransferOut := r.syncState.TransferOutAmount
		expectedTransferOut := fixedpoint.Zero
		for _, trade := range r.futuresWorker.Executor().AllTrades() {
			// consider only the closing trades, which are buy trades for short futures leg
			if trade.Side != types.SideTypeBuy {
				continue
			}
			amount := r.syncState.DirectionPolicy.TransferAmountFromFuturesTrade(trade)
			expectedTransferOut = expectedTransferOut.Add(amount)
		}
		transferDiff := expectedTransferOut.Sub(accTransferOut)
		r.logger.Debugf("rebalance closing: expected transfer out amount: %s, actual transfer out amount: %s, diff: %s",
			expectedTransferOut,
			accTransferOut,
			transferDiff,
		)
		futuresBalance := futuresAccount.Balances()[baseAsset]
		maxWithdraw := futuresBalance.MaxWithdrawAmount
		if maxWithdraw != nil && maxWithdraw.Sign() > 0 {
			transferDiff = fixedpoint.Min(transferDiff, *maxWithdraw)
		}
		if transferDiff.Sign() <= 0 {
			return nil
		}
		// 3. transfer the base asset from futures to spot
		if err := r.futuresService.TransferFuturesAccountAsset(timedCtx, baseAsset, transferDiff, types.TransferOut); err != nil {
			return fmt.Errorf("failed to transfer %s %s from futures to spot: %w", transferDiff, baseAsset, err)
		}
		r.syncState.TransferOutAmount = r.syncState.TransferOutAmount.Add(transferDiff)
	} else {
		// TODO: rebalance the long futures leg when closing
	}
	return nil
}
