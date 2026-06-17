package xfundingv2

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func (w *TWAPWorker) Initialize(ctx context.Context, s *Strategy) error {
	if w.syncState.TWAPExecutor == nil {
		// should not happen
		return fmt.Errorf("[TWAPWorker] TWAPExecutor is nil")
	}

	w.ctx = ctx
	w.SetLogger(s.logger)
	if err := w.syncState.TWAPExecutor.Initialize(s); err != nil {
		return fmt.Errorf("[TWAPWorker] failed to load TWAPExecutor: %w", err)
	}
	if w.syncState.ActiveOrder != nil {
		return w.syncState.TWAPExecutor.SyncOrder(*w.syncState.ActiveOrder)
	}
	return nil
}

type TWAPWorkerSyncState struct {
	Config TWAPWorkerConfig `json:"config"`

	// TargetPosition: positive = buy/long, negative = sell/short
	TargetPosition       fixedpoint.Value `json:"targetPosition"`
	State                TWAPWorkerState  `json:"state"`
	StartTime            time.Time        `json:"startTime"`
	EndTime              time.Time        `json:"endTime"`
	CurrentIntervalStart time.Time        `json:"currentIntervalStart"`
	CurrentIntervalEnd   time.Time        `json:"currentIntervalEnd"`
	LastCheckTime        time.Time        `json:"lastCheckTime"`
	PlaceOrderInterval   time.Duration    `json:"placeOrderInterval"`

	Symbol       string        `json:"symbol"`
	ActiveOrder  *types.Order  `json:"activeOrder,omitempty"`
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
