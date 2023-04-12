package backoff

import (
	"context"
	"time"

	"github.com/cenkalti/backoff/v4"
)

var MaxRetries uint64 = 101

// RetryGeneral retries operation with max retry times 101 and with the exponential backoff
func RetryGeneral(parent context.Context, op backoff.Operation) (err error) {
	ctx, cancel := context.WithTimeout(parent, 15*time.Minute)
	defer cancel()

	err = backoff.Retry(op, backoff.WithContext(
		backoff.WithMaxRetries(
			backoff.NewExponentialBackOff(),
			MaxRetries),
		ctx))
	return err
}
