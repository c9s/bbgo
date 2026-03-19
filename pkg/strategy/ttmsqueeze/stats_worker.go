package ttmsqueeze

import (
	"context"
	"sync"
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

// StateRecord represents a time period for a state
type StateRecord struct {
	State     State
	NextState State
	StartTime time.Time
	EndTime   time.Time
}

// Duration returns the duration of the state record
func (r *StateRecord) Duration() time.Duration {
	return r.EndTime.Sub(r.StartTime)
}

// StatsWorker tracks state transitions and records time periods
type StatsWorker struct {
	RecordC       chan *StateRecord
	HardExitC     chan time.Time
	BasePositionC chan fixedpoint.Value

	// current state tracking
	currentState     State
	currentStartTime time.Time

	stateRecords  []*StateRecord
	hotExitTimes  []time.Time
	basePositions []fixedpoint.Value
	runOnce       sync.Once
	stopOnce      sync.Once
}

// NewRecordWorker creates a new state recorder
func NewRecordWorker(currentState State) *StatsWorker {
	return &StatsWorker{
		RecordC:       make(chan *StateRecord, 100),
		HardExitC:     make(chan time.Time, 100),
		BasePositionC: make(chan fixedpoint.Value, 100),

		currentState:     currentState,
		currentStartTime: time.Now(),

		stateRecords:  make([]*StateRecord, 0),
		hotExitTimes:  make([]time.Time, 0),
		basePositions: make([]fixedpoint.Value, 0),
	}
}

func (r *StatsWorker) GetStateRecords() []*StateRecord {
	return r.stateRecords
}

func (r *StatsWorker) GetHotExitTimes() []time.Time {
	return r.hotExitTimes
}

func (r *StatsWorker) GetBasePositions() []fixedpoint.Value {
	return r.basePositions
}

func (r *StatsWorker) Run(wg *sync.WaitGroup, ctx context.Context) {
	r.runOnce.Do(func() {
		go r.run(wg, ctx)
	})
}

func (r *StatsWorker) run(wg *sync.WaitGroup, ctx context.Context) {
	wg.Add(1)
	defer wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case record, ok := <-r.RecordC:
			if !ok {
				return
			}
			record.StartTime = r.currentStartTime

			r.currentState = record.NextState
			r.currentStartTime = record.EndTime

			r.stateRecords = append(r.stateRecords, record)

			// Truncate if over 100 records
			if len(r.stateRecords) > 100 {
				r.stateRecords = r.stateRecords[len(r.stateRecords)-50:]
			}
		case exitTime, ok := <-r.HardExitC:
			if !ok {
				return
			}
			r.hotExitTimes = append(r.hotExitTimes, exitTime)

			// Truncate if over 100 records
			if len(r.hotExitTimes) > 100 {
				r.hotExitTimes = r.hotExitTimes[len(r.hotExitTimes)-50:]
			}
		case basePos, ok := <-r.BasePositionC:
			if !ok {
				return
			}
			r.basePositions = append(r.basePositions, basePos)

			// Truncate if over 100 records
			if len(r.basePositions) > 100 {
				r.basePositions = r.basePositions[len(r.basePositions)-50:]
			}
		}
	}
}

func (r *StatsWorker) Stop() {
	r.stopOnce.Do(r.stop)
}

func (r *StatsWorker) stop() {
	r.stateRecords = append(
		r.stateRecords,
		&StateRecord{
			State:     r.currentState,
			NextState: r.currentState,
			StartTime: r.currentStartTime,
			EndTime:   time.Now(),
		},
	)
	close(r.RecordC)
	close(r.HardExitC)
}
