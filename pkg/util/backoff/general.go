package backoff

import (
	"context"

	"github.com/cenkalti/backoff/v4"
)

var MaxRetries uint64 = 101

func RetryGeneral(ctx context.Context, op backoff.Operation) (err error) {
	err = backoff.Retry(op, backoff.WithContext(
		backoff.WithMaxRetries(
			backoff.NewExponentialBackOff(),
			MaxRetries),
		ctx))
	return err
}
