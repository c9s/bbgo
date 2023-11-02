package batch

import "time"

type Option func(query *AsyncTimeRangedBatchQuery)

// JumpIfEmpty jump the startTime + duration when the result is empty
func JumpIfEmpty(duration time.Duration) Option {
	return func(query *AsyncTimeRangedBatchQuery) {
		query.JumpIfEmpty = duration
	}
}
