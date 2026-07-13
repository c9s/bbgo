package xfundingv2

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func (r *ArbitrageRound) Initialize(ctx context.Context, s *Strategy) error {
	r.SetLogger(s.logger)
	if s.futuresSession.Exchange.Name() != r.syncState.FuturesExchangeName {
		return fmt.Errorf("[ArbitrageRound] futures exchange name mismatch: expected %s, got %s",
			r.syncState.FuturesExchangeName, s.futuresSession.Exchange.Name())
	}
	r.SetFuturesExchangeFeeRates(
		types.ExchangeFee{
			MakerFeeRate: s.futuresSession.MakerFeeRate,
			TakerFeeRate: s.futuresSession.TakerFeeRate,
		},
	)
	if s.spotSession.Exchange.Name() != r.syncState.SpotExchangeName {
		return fmt.Errorf("[ArbitrageRound] spot exchange name mismatch: expected %s, got %s",
			r.syncState.SpotExchangeName, s.spotSession.Exchange.Name())
	}
	r.SetSpotExchangeFeeRates(
		types.ExchangeFee{
			MakerFeeRate: s.spotSession.MakerFeeRate,
			TakerFeeRate: s.spotSession.TakerFeeRate,
		},
	)
	r.retryTransferTickC = make(chan time.Time, 100)
	if r.hasStarted() {
		// the round has been started before, we need to start the retry worker
		go r.retryTransferWorker(ctx, r.retryTransferTickC)
		r.spotSession = s.spotSession
		r.futuresSession = s.futuresSession
	}
	if service, ok := s.futuresSession.Exchange.(FuturesService); ok {
		r.futuresService = service
	} else {
		return errors.New("[ArbitrageRound] futures exchange does not implement FuturesService")
	}
	if r.spotWorker != nil {
		if err := r.spotWorker.Initialize(ctx, s); err != nil {
			return fmt.Errorf("[ArbitrageRound] spot load strategy error: %w", err)
		}
	} else {
		// should not happend
		// by the time we create the round, the spot worker is never nil
		// the restored round should always have the spot worker restored as well.
		return errors.New("[ArbitrageRound] spot worker is nil")
	}
	if r.futuresWorker != nil {
		if err := r.futuresWorker.Initialize(ctx, s); err != nil {
			return fmt.Errorf("[ArbitrageRound] futures load strategy error: %w", err)
		}
	} else {
		// should not happend
		// by the time we create the round, the futures worker is never nil
		// the restored round should always have the futures worker restored as well.
		return errors.New("[ArbitrageRound] futures worker is nil")
	}
	r.rebalanceInterval = s.RoundRebalanceInterval.Duration()

	return nil
}

type ArbitrageRoundSyncState struct {
	ID string `json:"id"`

	TriggeredFundingRate        fixedpoint.Value     `json:"triggeredFundingRate"`
	TriggeredSpotTargetPosition fixedpoint.Value     `json:"triggeredSpotTargetPosition"`
	TransferInAmount            fixedpoint.Value     `json:"transferInAmount"`
	TransferOutAmount           fixedpoint.Value     `json:"transferOutAmount"`
	MinHoldingIntervals         int                  `json:"minHoldingIntervals"`
	FundingIntervalHours        int                  `json:"fundingIntervalHours"`
	Leverage                    fixedpoint.Value     `json:"leverage"`
	FundingIntervalStart        time.Time            `json:"fundingIntervalStart"`
	FundingIntervalEnd          time.Time            `json:"fundingIntervalEnd"`
	FundingFeeRecords           map[int64]FundingFee `json:"fundingFeeRecords"`

	Symbol              string             `json:"symbol"`
	SpotExchangeName    types.ExchangeName `json:"spotExchangeName"`
	FuturesExchangeName types.ExchangeName `json:"futuresExchangeName"`
	DirectionPolicy     directionPolicy    `json:"directionPolicy"`

	SpotFeeAssetAmount    fixedpoint.Value `json:"spotFeeAssetAmount"`
	FuturesFeeAssetAmount fixedpoint.Value `json:"futuresFeeAssetAmount"`
	FeeSymbol             string           `json:"feeSymbol"`
	AvgFeeCost            fixedpoint.Value `json:"avgFeeCost"`

	RetryDuration       time.Duration             `json:"retryDuration"`
	RetryTransfers      map[uint64]*transferRetry `json:"retryTransfers"`
	SyncedSpotTrades    map[uint64]struct{}       `json:"syncedSpotTrades"`
	SyncedFuturesTrades map[uint64]struct{}       `json:"syncedFuturesTrades"`

	State RoundState `json:"state"`

	// StartAt is the time when the round is started
	StartAt time.Time `json:"startAt"`
	// ClosingAt is the time when the round is entered closing state
	ClosingAt       time.Time      `json:"closingAt"`
	ClosingDuration types.Duration `json:"closingDuration"`
	// LastUpdateTime is the last time when the round is updated
	LastUpdateTime time.Time `json:"lastUpdateTime"`

	// ReadyAt is the time when the round enters ready state
	ReadyAt time.Time `json:"readyAt"`

	// ClosedAt is the time when the round is closed
	ClosedAt time.Time `json:"closedAt"`

	LargeDeviationStartTime time.Time `json:"largeDeviationStartTime"`
}

func (r *ArbitrageRound) MarshalJSON() ([]byte, error) {
	v := struct {
		SyncState     ArbitrageRoundSyncState `json:"syncState"`
		SpotWorker    *TWAPWorker             `json:"spotWorker,omitempty"`
		FuturesWorker *TWAPWorker             `json:"futuresWorker,omitempty"`
	}{
		SyncState:     r.syncState,
		SpotWorker:    r.spotWorker,
		FuturesWorker: r.futuresWorker,
	}
	return json.Marshal(&v)
}

func (r *ArbitrageRound) UnmarshalJSON(b []byte) error {
	v := struct {
		SyncState     ArbitrageRoundSyncState `json:"syncState"`
		SpotWorker    *TWAPWorker             `json:"spotWorker,omitempty"`
		FuturesWorker *TWAPWorker             `json:"futuresWorker,omitempty"`
	}{}
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	r.syncState = v.SyncState
	r.spotWorker = v.SpotWorker
	r.futuresWorker = v.FuturesWorker
	return nil
}
