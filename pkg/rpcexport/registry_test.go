package rpcexport

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// drainOnce simulates a strategy's Run() loop servicing exactly one
// pending call from its inbox.
func drainOnce(t *testing.T, r *Registry) {
	t.Helper()
	select {
	case call := <-r.Requests():
		r.Dispatch(call)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for a call to drain")
	}
}

func TestRegistry_ExportAndInvoke_RoundTrip(t *testing.T) {
	r := NewRegistry("test-instance")
	r.Export("add", "adds two numbers", false, func(a, b int) (int, error) {
		return a + b, nil
	}, "a", "b")

	methods := r.Methods()
	require.Len(t, methods, 1)
	assert.Equal(t, "add", methods[0].Name)
	assert.False(t, methods[0].Mutating)
	require.Len(t, methods[0].Params, 2)
	assert.Equal(t, "a", methods[0].Params[0].Name)
	assert.Equal(t, "int", methods[0].Params[0].Type)

	resultCh, err := r.Enqueue(context.Background(), "add", json.RawMessage(`[2, 3]`))
	require.NoError(t, err)

	drainOnce(t, r)

	res := <-resultCh
	require.NoError(t, res.Err)

	var sum int
	require.NoError(t, json.Unmarshal(res.Data, &sum))
	assert.Equal(t, 5, sum)
}

func TestRegistry_Invoke_ErrorReturn(t *testing.T) {
	r := NewRegistry("test-instance")
	wantErr := errors.New("boom")
	r.Export("fail", "always fails", true, func() error {
		return wantErr
	})

	resultCh, err := r.Enqueue(context.Background(), "fail", json.RawMessage(`[]`))
	require.NoError(t, err)

	drainOnce(t, r)

	res := <-resultCh
	require.Error(t, res.Err)
	assert.Contains(t, res.Err.Error(), "boom")
	assert.Nil(t, res.Data)
}

func TestRegistry_Enqueue_UnknownMethod(t *testing.T) {
	r := NewRegistry("test-instance")

	_, err := r.Enqueue(context.Background(), "nope", json.RawMessage(`[]`))
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrMethodNotFound)
}

func TestRegistry_Dispatch_PanicRecovered(t *testing.T) {
	r := NewRegistry("test-instance")
	r.Export("explode", "panics", true, func() error {
		panic("kaboom")
	})

	resultCh, err := r.Enqueue(context.Background(), "explode", json.RawMessage(`[]`))
	require.NoError(t, err)

	assert.NotPanics(t, func() {
		drainOnce(t, r)
	})

	res := <-resultCh
	require.Error(t, res.Err)
	assert.Contains(t, res.Err.Error(), "kaboom")
}

func TestRegistry_Enqueue_InboxFull(t *testing.T) {
	r := NewRegistry("test-instance")
	r.Export("noop", "does nothing", false, func() error { return nil })

	// Fill the inbox without ever draining it.
	for i := 0; i < defaultInboxSize; i++ {
		_, err := r.Enqueue(context.Background(), "noop", json.RawMessage(`[]`))
		require.NoError(t, err)
	}

	_, err := r.Enqueue(context.Background(), "noop", json.RawMessage(`[]`))
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInboxFull)
}

func TestRegistry_Invoke_WrongParamCount(t *testing.T) {
	r := NewRegistry("test-instance")
	r.Export("needsOne", "needs one param", false, func(s string) error { return nil })

	resultCh, err := r.Enqueue(context.Background(), "needsOne", json.RawMessage(`[]`))
	require.NoError(t, err)

	drainOnce(t, r)

	res := <-resultCh
	require.Error(t, res.Err)
}
