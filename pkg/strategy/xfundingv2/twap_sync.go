package xfundingv2

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

func (w *TWAPWorker) Initialize(ctx context.Context, s *Strategy) error {
	if w.syncState.TWAPExecutor == nil {
		// should not happen
		return fmt.Errorf("[TWAPWorker] TWAPExecutor is nil")
	}

	w.ctx = ctx
	if w.syncState.TWAPExecutor.IsFutures() {
		w.account = s.futuresSession.Account
	} else {
		w.account = s.spotSession.Account
	}
	w.SetLogger(s.logger)
	if err := w.syncState.TWAPExecutor.Initialize(ctx, s); err != nil {
		return fmt.Errorf("[TWAPWorker] failed to load TWAPExecutor: %w", err)
	}
	return nil
}

type TWAPWorkerSyncState struct {
	Config TWAPWorkerConfig `json:"config"`

	// TargetPosition: positive = buy/long, negative = sell/short
	TargetPosition       fixedpoint.Value `json:"targetPosition"`
	State                TWAPWorkerState  `json:"state"`
	StartAt              time.Time        `json:"startAt"`
	EndAt                time.Time        `json:"endAt"`
	CurrentIntervalStart time.Time        `json:"currentIntervalStart"`
	CurrentIntervalEnd   time.Time        `json:"currentIntervalEnd"`
	LastCheckTime        time.Time        `json:"lastCheckTime"`
	PlaceOrderInterval   time.Duration    `json:"placeOrderInterval"`

	Symbol       string        `json:"symbol"`
	TWAPExecutor *TWAPExecutor `json:"executor,omitempty"`
}

func (w *TWAPWorker) MarshalJSON() ([]byte, error) {
	return json.Marshal(&w.syncState)
}

func (w *TWAPWorker) UnmarshalJSON(b []byte) error {
	stateData := TWAPWorkerSyncState{}
	if err := json.Unmarshal(b, &stateData); err != nil {
		return fmt.Errorf("failed to unmarshal TWAPWorker: %w", err)
	}

	w.syncState = stateData

	return nil
}
