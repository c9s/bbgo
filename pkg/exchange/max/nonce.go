package max

import (
	"context"
	"errors"
	"time"

	"github.com/c9s/requestgen"
	"github.com/cenkalti/backoff/v4"

	maxapi "github.com/c9s/bbgo/pkg/exchange/max/maxapi"
)

// isMaxNonceError reports whether err is a MAX nonce-gate rejection:
//
//	2006 = the nonce has already been used by access key
//	2007 = invalid nonce
//
// These are rejected during nonce validation, before the request reaches the
// matching/cancel engine, so resending with a fresh nonce is safe (no duplicate
// action).
func isMaxNonceError(err error) bool {
	var responseErr *requestgen.ErrResponse
	if !errors.As(err, &responseErr) {
		return false
	}

	if responseErr.StatusCode < 400 || !responseErr.IsJSON() {
		return false
	}

	var maxErr maxapi.ErrorResponse
	if decodeErr := responseErr.DecodeJSON(&maxErr); decodeErr != nil {
		return false
	}

	return maxErr.Err.Code == 2006 || maxErr.Err.Code == 2007
}

// retryOnNonceError re-invokes op with a fresh nonce when MAX rejects the
// request with a nonce-gate error (2006/2007). Any other error (or success)
// stops immediately; the original error is returned unwrapped.
func retryOnNonceError(ctx context.Context, op func() error) error {
	timedCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	return backoff.Retry(func() error {
		if err := op(); err != nil {
			if isMaxNonceError(err) {
				// retry with a fresh nonce
				return err
			}

			// any other error stops immediately, returned unwrapped
			return backoff.Permanent(err)
		}

		return nil
	}, backoff.WithContext(
		backoff.WithMaxRetries(
			backoff.NewExponentialBackOff(),
			3),
		timedCtx))
}
