package sigchan

import "time"

type Chan chan struct{}

func New(cap int) Chan {
	return make(Chan, cap)
}

func (c Chan) Drain(duration, deadline time.Duration) (cnt int) {
	cnt = 0

	deadlineC := time.After(deadline)
	for {
		select {
		case <-c:
			cnt++

		case <-deadlineC:
			return cnt

		case <-time.After(duration):
			return cnt
		}
	}
}

func (c Chan) Emit() {
	select {
	case c <- struct{}{}:
	default:
	}
}

func (c Chan) Close() {
	close(c)
}
