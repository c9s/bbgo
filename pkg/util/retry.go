package util

import (
	"context"
	"time"

	"github.com/pkg/errors"
)

const (
	InfiniteRetry = 0
)

type RetryPredicator func(e error) bool

// Retry retrys the passed function for "attempts" times, if passed function return error. Setting attempts to zero means keep retrying.
func Retry(ctx context.Context, attempts int, duration time.Duration, fnToRetry func() error, errHandler func(error), predicators ...RetryPredicator) (err error) {
	infinite := false
	if attempts == InfiniteRetry {
		infinite = true
	}

	for attempts > 0 || infinite {
		select {
		case <-ctx.Done():
			errMsg := "return for context done"
			if err != nil {
				return errors.Wrap(err, errMsg)
			} else {
				return errors.New(errMsg)
			}
		default:
			if err = fnToRetry(); err == nil {
				return nil
			}

			if !needRetry(err, predicators) {
				return err
			}

			err = errors.Wrapf(err, "failed in retry: countdown: %v", attempts)

			if errHandler != nil {
				errHandler(err)
			}

			if !infinite {
				attempts--
			}

			time.Sleep(duration)
		}
	}

	return err
}

func needRetry(err error, predicators []RetryPredicator) bool {
	if err == nil {
		return false
	}

	// If no predicators specified, we will retry for all errors
	if len(predicators) == 0 {
		return true
	}

	return predicators[0](err)
}
