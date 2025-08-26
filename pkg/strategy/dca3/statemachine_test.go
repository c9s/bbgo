package dca3

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestNewStateMachine(t *testing.T) {
	logger := logrus.NewEntry(logrus.New())
	sm := NewStateMachine(logger)

	assert.NotNil(t, sm)
	assert.Equal(t, None, sm.GetState())
	assert.NotNil(t, sm.nextStateC)
	assert.NotNil(t, sm.closeC)
	assert.False(t, sm.isRunning.Load())
}

func TestStateMachine_GetState(t *testing.T) {
	sm := NewStateMachine(nil)

	// Initial state should be None
	assert.Equal(t, None, sm.GetState())

	// Update state and verify
	sm.UpdateState(StateIdleWaiting)
	assert.Equal(t, StateIdleWaiting, sm.GetState())
}

func TestStateMachine_UpdateState(t *testing.T) {
	sm := NewStateMachine(nil)

	// Test state transitions
	testStates := []State{StateIdleWaiting, StateOpenPositionReady, StateOpenPositionMOQReached, StateTakeProfitOrderReset, StateTakeProfitReached}

	for _, state := range testStates {
		sm.UpdateState(state)
		assert.Equal(t, state, sm.GetState())
	}
}

func TestStateMachine_RegisterTransitionFunc(t *testing.T) {
	sm := NewStateMachine(nil)

	callCount := 0
	transitionFunc := func(ctx context.Context) (error, LogLevel) {
		callCount++
		return nil, LogLevelNone
	}

	// Register transition function
	sm.RegisterTransitionFunc(StateIdleWaiting, StateOpenPositionReady, transitionFunc)

	// Verify transition function is registered
	assert.NotNil(t, sm.stateTransitionFunc[StateIdleWaiting][StateOpenPositionReady])

	// Test transition function call
	err, _ := sm.stateTransitionFunc[StateIdleWaiting][StateOpenPositionReady](context.Background())
	assert.NoError(t, err)
	assert.Equal(t, 1, callCount)
}

func TestStateMachine_EmitNextState(t *testing.T) {
	sm := NewStateMachine(nil)

	// Emit state to channel
	sm.EmitNextState(StateOpenPositionReady)

	// Verify state is in channel
	select {
	case state := <-sm.nextStateC:
		assert.Equal(t, StateOpenPositionReady, state)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Expected state in channel")
	}
}

func TestStateMachine_EmitNextState_ChannelFull(t *testing.T) {
	sm := NewStateMachine(nil)

	// Fill the channel
	sm.EmitNextState(StateOpenPositionReady)

	// Try to emit another state (should not block)
	sm.EmitNextState(StateOpenPositionMOQReached)

	// Only first state should be in channel
	select {
	case state := <-sm.nextStateC:
		assert.Equal(t, StateOpenPositionReady, state)
	default:
		t.Fatal("Expected first state in channel")
	}

	// Channel should be empty now
	select {
	case <-sm.nextStateC:
		t.Fatal("Channel should be empty")
	default:
		// Expected
	}
}

func TestStateMachine_RunAndClose(t *testing.T) {
	sm := NewStateMachine(nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	startCalled := false
	closeCalled := false

	sm.OnStart(func() {
		startCalled = true
	})

	sm.OnClose(func() {
		closeCalled = true
	})

	// Start state machine
	sm.Run(ctx)

	// Wait for start
	time.Sleep(50 * time.Millisecond)
	assert.True(t, sm.isRunning.Load())
	assert.True(t, startCalled)

	// Close state machine
	sm.Close()

	// Wait for close
	time.Sleep(50 * time.Millisecond)
	assert.False(t, sm.isRunning.Load())
	assert.True(t, closeCalled)
}

func TestStateMachine_StateTransition(t *testing.T) {
	sm := NewStateMachine(nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var transitionCalled atomic.Bool

	// Register transition function
	sm.RegisterTransitionFunc(StateIdleWaiting, StateOpenPositionReady, func(ctx context.Context) (error, LogLevel) {
		transitionCalled.Store(true)
		return nil, LogLevelNone
	})

	// Set initial state
	sm.UpdateState(StateIdleWaiting)

	// Start state machine
	sm.Run(ctx)
	time.Sleep(50 * time.Millisecond)

	// Emit next state
	sm.EmitNextState(StateOpenPositionReady)

	// Wait for transition
	time.Sleep(100 * time.Millisecond)

	assert.True(t, transitionCalled.Load())
	assert.Equal(t, StateOpenPositionReady, sm.GetState())

	sm.Close()
}

func TestStateMachine_StateTransitionError(t *testing.T) {
	sm := NewStateMachine(nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	expectedError := fmt.Errorf("transition error")

	// Register transition function that returns error
	sm.RegisterTransitionFunc(StateIdleWaiting, StateOpenPositionReady, func(ctx context.Context) (error, LogLevel) {
		return expectedError, LogLevelError
	})

	// Set initial state
	sm.UpdateState(StateIdleWaiting)

	// Start state machine
	sm.Run(ctx)
	time.Sleep(50 * time.Millisecond)

	// Emit next state
	sm.EmitNextState(StateOpenPositionReady)

	// Wait for transition attempt
	time.Sleep(100 * time.Millisecond)

	// State should remain unchanged due to error
	assert.Equal(t, StateIdleWaiting, sm.GetState())

	sm.Close()
}

func TestStateMachine_NoTransitionFunction(t *testing.T) {
	sm := NewStateMachine(nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set initial state
	sm.UpdateState(StateIdleWaiting)

	// Start state machine
	sm.Run(ctx)
	time.Sleep(50 * time.Millisecond)

	// Emit state transition without registered function
	sm.EmitNextState(StateOpenPositionReady)

	// Wait for transition attempt
	time.Sleep(100 * time.Millisecond)

	// State should remain unchanged
	assert.Equal(t, StateIdleWaiting, sm.GetState())

	sm.Close()
}

func TestStateMachine_WaitForRunningIs(t *testing.T) {
	sm := NewStateMachine(nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Test waiting for running = true
	go func() {
		time.Sleep(50 * time.Millisecond)
		sm.Run(ctx)
	}()

	isRunning := sm.WaitForRunningIs(true, 10*time.Millisecond, 200*time.Millisecond)
	assert.True(t, isRunning)

	// Test waiting for running = false
	go func() {
		time.Sleep(50 * time.Millisecond)
		sm.Close()
	}()

	isNotRunning := sm.WaitForRunningIs(false, 10*time.Millisecond, 200*time.Millisecond)
	assert.True(t, isNotRunning)
}

func TestStateMachine_WaitForRunningIs_Timeout(t *testing.T) {
	sm := NewStateMachine(nil)

	// Test timeout when waiting for running = true
	isRunning := sm.WaitForRunningIs(true, 10*time.Millisecond, 50*time.Millisecond)
	assert.False(t, isRunning)
}

func TestStateMachine_ContextCancellation(t *testing.T) {
	sm := NewStateMachine(nil)
	ctx, cancel := context.WithCancel(context.Background())

	// Start state machine
	sm.Run(ctx)
	time.Sleep(50 * time.Millisecond)
	assert.True(t, sm.isRunning.Load())

	// Cancel context
	cancel()

	// Wait for state machine to stop
	time.Sleep(100 * time.Millisecond)
	assert.False(t, sm.isRunning.Load())
}

func TestStateMachine_MultipleCallbacks(t *testing.T) {
	sm := NewStateMachine(nil)

	var startCallCount atomic.Int64
	var closeCallCount atomic.Int64

	// Register multiple start callbacks
	sm.OnStart(func() { startCallCount.Add(1) })
	sm.OnStart(func() { startCallCount.Add(1) })

	// Register multiple close callbacks
	sm.OnClose(func() { closeCallCount.Add(1) })
	sm.OnClose(func() { closeCallCount.Add(1) })

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start and close state machine
	sm.Run(ctx)
	time.Sleep(50 * time.Millisecond)
	sm.Close()
	time.Sleep(50 * time.Millisecond)

	// All callbacks should be called
	assert.Equal(t, int64(2), startCallCount.Load())
	assert.Equal(t, int64(2), closeCallCount.Load())
}
