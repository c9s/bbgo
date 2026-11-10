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

func TestRegistry_Invoke_RoundTrip(t *testing.T) {
	r := NewRegistry()
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

	data, err := r.Invoke(context.Background(), "add", json.RawMessage(`[2, 3]`))
	require.NoError(t, err)

	var sum int
	require.NoError(t, json.Unmarshal(data, &sum))
	assert.Equal(t, 5, sum)
}

func TestRegistry_Invoke_ErrorReturn(t *testing.T) {
	r := NewRegistry()
	wantErr := errors.New("boom")
	r.Export("fail", "always fails", true, func() error {
		return wantErr
	})

	data, err := r.Invoke(context.Background(), "fail", json.RawMessage(`[]`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "boom")
	assert.Nil(t, data)
}

func TestRegistry_Invoke_UnknownMethod(t *testing.T) {
	r := NewRegistry()

	_, err := r.Invoke(context.Background(), "nope", json.RawMessage(`[]`))
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrMethodNotFound)
}

func TestRegistry_Invoke_PanicRecovered(t *testing.T) {
	r := NewRegistry()
	r.Export("explode", "panics", true, func() error {
		panic("kaboom")
	})

	var data json.RawMessage
	var err error
	assert.NotPanics(t, func() {
		data, err = r.Invoke(context.Background(), "explode", json.RawMessage(`[]`))
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "kaboom")
	assert.Nil(t, data)
}

func TestRegistry_Invoke_WrongParamCount(t *testing.T) {
	r := NewRegistry()
	r.Export("needsOne", "needs one param", false, func(s string) error { return nil })

	_, err := r.Invoke(context.Background(), "needsOne", json.RawMessage(`[]`))
	require.Error(t, err)
}

func TestRegistry_Invoke_DeadlineExceeded(t *testing.T) {
	r := NewRegistry()
	unblock := make(chan struct{})
	t.Cleanup(func() { close(unblock) })

	r.Export("stall", "never returns in time", false, func() error {
		<-unblock
		return nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := r.Invoke(ctx, "stall", json.RawMessage(`[]`))
	require.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}
