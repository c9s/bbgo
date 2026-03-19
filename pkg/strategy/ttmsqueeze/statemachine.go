package ttmsqueeze

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	twap "github.com/c9s/bbgo/pkg/twap/v2"
	"github.com/c9s/bbgo/pkg/types"
)

// TwapConfig contains TWAP execution settings
type TwapConfig struct {
	// UpdateInterval is the update interval for TWAP orders
	UpdateInterval types.Duration `json:"updateInterval"`

	// NumOfTicks is the number of price ticks behind the best bid/ask
	NumOfTicks int `json:"numOfTicks"`

	// Deadline is the maximum duration for entry TWAP execution
	Deadline types.Duration `json:"deadline"`
}

func (c *TwapConfig) Defaults() {
	if c.UpdateInterval == 0 {
		c.UpdateInterval = types.Duration(10 * time.Second)
	}
	if c.NumOfTicks == 0 {
		c.NumOfTicks = 2
	}
	if c.Deadline == 0 {
		c.Deadline = types.Duration(5 * time.Minute) // default 5 minutes, ensures market order fallback
	}
}

// StateMachine manages the strategy state and TWAP execution
// It ensures only one TWAP executor runs at a time
//
//go:generate callbackgen -type=StateMachine
type StateMachine struct {
	mu    sync.Mutex
	state State

	// TWAP executor - only one at a time
	twapExecutor    *twap.FixedQuantityExecutor
	currentTwapSide types.SideType

	// Dependencies
	exchange   types.Exchange
	symbol     string
	market     types.Market
	twapConfig *TwapConfig
	position   *types.Position

	logger *logrus.Entry

	stateChangeCallbacks []func(from, to State)
}

func (sm *StateMachine) String() string {
	twapStatus := "none"
	if sm.twapExecutor != nil {
		twapStatus = fmt.Sprintf("%s TWAP running", sm.currentTwapSide)
	}

	return fmt.Sprintf("StateMachine(state=%s, twap=%s)", sm.state, twapStatus)
}

// NewStateMachine creates a new state machine for the strategy
func NewStateMachine(
	exchange types.Exchange,
	symbol string,
	market types.Market,
	twapConfig *TwapConfig,
	position *types.Position,
	logger *logrus.Entry,
) *StateMachine {
	return &StateMachine{
		state:      StateIdle,
		exchange:   exchange,
		symbol:     symbol,
		market:     market,
		twapConfig: twapConfig,
		position:   position,
		logger:     logger.WithField("component", "statemachine"),
	}
}

// GetState returns the current state (thread-safe)
func (sm *StateMachine) GetState() State {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	return sm.state
}

// IsTwapRunning returns true if a TWAP executor is currently running
func (sm *StateMachine) IsTwapRunning() bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	return sm.twapExecutor != nil
}

// GetTwapSide returns the current TWAP side (Buy or Sell)
func (sm *StateMachine) GetTwapSide() types.SideType {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	return sm.currentTwapSide
}

// TransitionToLong handles Idle -> Long or Exiting -> Long (re-enter)
// Starts a buy TWAP executor
func (sm *StateMachine) TransitionToLong(ctx context.Context, quantity, sliceQuantity fixedpoint.Value) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// If active buy TWAP, skip
	if sm.twapExecutor != nil && sm.currentTwapSide == types.SideTypeBuy {
		return nil
	}

	// If there's a sell TWAP running (e.g., re-entering from Exiting), stop it first
	if sm.twapExecutor != nil && sm.currentTwapSide == types.SideTypeSell {
		sm.logger.Info("stopping sell TWAP to re-enter long position")
		sm.shutdownTwapLocked(ctx)
	}

	// Start buy TWAP to open/increase long position
	if err := sm.startTwapLocked(ctx, types.SideTypeBuy, quantity, sliceQuantity, sm.twapConfig.UpdateInterval.Duration(), sm.twapConfig.Deadline.Duration()); err != nil {
		return err
	}
	// Transition to Long state
	prevState := sm.state
	sm.state = StateLong
	if prevState != sm.state {
		sm.EmitStateChange(prevState, sm.state)
	}

	return nil
}

// TransitionToExiting handles Long -> Exiting (exit position)
// Stops any running TWAP and starts a sell TWAP to close the entire position
// updateInterval specifies the interval between TWAP orders
// deadline specifies the maximum duration for the TWAP to complete
func (sm *StateMachine) TransitionToExiting(ctx context.Context, quantity, sliceQuantity fixedpoint.Value, updateInterval, deadline time.Duration) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.state == StateExiting {
		return nil
	}

	if sm.state == StateIdle {
		return fmt.Errorf("cannot exit from Idle state (no position)")
	}

	// Stop any running TWAP
	sm.shutdownTwapLocked(ctx)

	// Start sell TWAP to exit position
	if err := sm.startTwapLocked(ctx, types.SideTypeSell, quantity, sliceQuantity, updateInterval, deadline); err != nil {
		return err
	}
	// Transition to Exiting state
	prevState := sm.state
	sm.state = StateExiting
	if prevState != sm.state {
		sm.EmitStateChange(prevState, sm.state)
	}
	return nil
}

// transitionToIdleLocked transitions to Idle state (must hold lock)
func (sm *StateMachine) transitionToIdleLocked() {
	prevState := sm.state
	sm.state = StateIdle
	sm.resetTwapLocked()
	if prevState != sm.state {
		sm.EmitStateChange(prevState, sm.state)
	}
}

// CheckAndUpdateState checks position and updates state if needed
// Call this after trades to detect when position is fully closed
func (sm *StateMachine) CheckAndUpdateState() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	isDust := sm.position.IsDust()

	switch sm.state {
	case StateExiting:
		// If position is dust, transition to Idle
		if isDust {
			sm.transitionToIdleLocked()
		}
	case StateLong:
		// If position becomes dust unexpectedly, go back to Idle
		if isDust {
			sm.transitionToIdleLocked()
		}
	case StateIdle:
		// If position appears while in Idle (e.g., manual trade), transition to Long
		if !isDust && sm.position.IsLong() {
			prevState := sm.state
			sm.state = StateLong
			if prevState != sm.state {
				sm.EmitStateChange(prevState, sm.state)
			}
		}
	}
}

// Shutdown gracefully shuts down the state machine and any running TWAP
func (sm *StateMachine) Shutdown(ctx context.Context) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.shutdownTwapLocked(ctx)
}

// CancelAndReset cancels any running TWAP and transitions to Idle state
// Used for hard exits where we need to immediately cancel any ongoing execution
func (sm *StateMachine) CancelAndReset(ctx context.Context) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.shutdownTwapLocked(ctx)
	if sm.state != StateIdle {
		sm.transitionToIdleLocked()
	}
}

// startTwapLocked starts a TWAP executor (must hold lock)
func (sm *StateMachine) startTwapLocked(ctx context.Context, side types.SideType, targetQuantity, sliceQuantity fixedpoint.Value, updateInterval, deadline time.Duration) error {
	executor := twap.NewFixedQuantityExecutor(
		sm.exchange,
		sm.symbol,
		sm.market,
		side,
		targetQuantity,
		sliceQuantity,
	)

	executor.SetUpdateInterval(updateInterval)
	executor.SetNumOfTicks(sm.twapConfig.NumOfTicks)

	if deadline > 0 {
		executor.SetDeadlineTime(time.Now().Add(deadline))
	}

	if err := executor.Start(ctx); err != nil {
		return fmt.Errorf("failed to start TWAP executor: %w", err)
	}

	sm.twapExecutor = executor
	sm.currentTwapSide = side
	sm.logger.Infof(
		"started %s TWAP: quantity=%s, slice=%s, interval=%s, deadline=%s",
		side,
		targetQuantity.String(),
		sliceQuantity.String(),
		updateInterval,
		deadline,
	)

	// Monitor TWAP completion in background
	go sm.monitorTwapDone(ctx)

	return nil
}

// shutdownTwapLocked shuts down the current TWAP executor (must hold lock)
func (sm *StateMachine) shutdownTwapLocked(ctx context.Context) {
	if sm.twapExecutor == nil {
		return
	}

	shutdownCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	sm.twapExecutor.Shutdown(shutdownCtx)
	sm.resetTwapLocked()
	sm.logger.Info("TWAP executor shutdown complete")
}

// resetTwapLocked resets the TWAP executor fields (must hold lock)
func (sm *StateMachine) resetTwapLocked() {
	sm.twapExecutor = nil
	sm.currentTwapSide = types.SideTypeNone
}

// monitorTwapDone monitors the TWAP executor for completion
func (sm *StateMachine) monitorTwapDone(ctx context.Context) {
	sm.mu.Lock()
	executor := sm.twapExecutor
	sm.mu.Unlock()

	if executor == nil {
		return
	}

	select {
	case <-ctx.Done():
		return
	case <-executor.Done():
		sm.mu.Lock()
		// Only clear if this is still the active executor
		if sm.twapExecutor == executor {
			sm.logger.Info("TWAP execution completed")
			sm.resetTwapLocked()
		}
		sm.mu.Unlock()
	}
}
