package max

import (
	"context"
	"io"
	"testing"

	"github.com/c9s/requestgen"
	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/testing/httptesting"
)

// newMaxErrResponse builds a *requestgen.ErrResponse carrying a JSON MAX error
// body ({"error":{"code":N,...}}) with the given HTTP status code.
func newMaxErrResponse(t *testing.T, statusCode, errCode int) *requestgen.ErrResponse {
	t.Helper()

	payload := map[string]any{
		"error": map[string]any{
			"code":    errCode,
			"message": "mock max error",
		},
	}

	resp, err := requestgen.NewResponse(httptesting.BuildResponseJson(statusCode, payload))
	assert.NoError(t, err)
	return &requestgen.ErrResponse{Response: resp, Body: resp.Body}
}

// newNonJSONErrResponse builds a *requestgen.ErrResponse with a non-JSON body.
func newNonJSONErrResponse(t *testing.T, statusCode int) *requestgen.ErrResponse {
	t.Helper()

	resp, err := requestgen.NewResponse(httptesting.BuildResponseString(statusCode, "internal server error"))
	assert.NoError(t, err)
	return &requestgen.ErrResponse{Response: resp, Body: resp.Body}
}

func TestIsMaxNonceError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "2006 nonce already used", err: newMaxErrResponse(t, 400, 2006), want: true},
		{name: "2007 invalid nonce", err: newMaxErrResponse(t, 400, 2007), want: true},
		{name: "2016 amount too small", err: newMaxErrResponse(t, 400, 2016), want: false},
		{name: "2018 can not lock fund", err: newMaxErrResponse(t, 400, 2018), want: false},
		{name: "non-4xx status", err: newMaxErrResponse(t, 200, 2006), want: false},
		{name: "non-JSON body", err: newNonJSONErrResponse(t, 500), want: false},
		{name: "not an ErrResponse", err: io.EOF, want: false},
		{name: "nil error", err: nil, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isMaxNonceError(tt.err))
		})
	}
}

func TestRetryOnNonceError_RetriesThenSucceeds(t *testing.T) {
	calls := 0
	err := retryOnNonceError(context.Background(), func() error {
		calls++
		if calls == 1 {
			return newMaxErrResponse(t, 400, 2006)
		}
		return nil
	})

	assert.NoError(t, err)
	assert.Equal(t, 2, calls, "should retry once with a fresh nonce and then succeed")
}

func TestRetryOnNonceError_StopsOnOtherError(t *testing.T) {
	t.Run("non-nonce max error", func(t *testing.T) {
		calls := 0
		nonNonceErr := newMaxErrResponse(t, 400, 2016)
		err := retryOnNonceError(context.Background(), func() error {
			calls++
			return nonNonceErr
		})

		assert.ErrorIs(t, err, nonNonceErr)
		assert.Equal(t, 1, calls, "non-nonce errors must not be retried")
	})

	t.Run("transport error", func(t *testing.T) {
		calls := 0
		err := retryOnNonceError(context.Background(), func() error {
			calls++
			return io.EOF
		})

		assert.ErrorIs(t, err, io.EOF)
		assert.Equal(t, 1, calls, "non-nonce errors must not be retried")
	})
}
