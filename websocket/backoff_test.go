package websocket

import (
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestBackoff(t *testing.T) {
	b := &Backoff{
		Min:    200 * time.Millisecond,
		Max:    100 * time.Second,
		Factor: 2,
	}

	equals(t, b.Duration(), 200*time.Millisecond)
	equals(t, b.Duration(), 400*time.Millisecond)
	equals(t, b.Duration(), 800*time.Millisecond)
	b.Reset()
	equals(t, b.Duration(), 200*time.Millisecond)
}

func TestReachMaxBackoff(t *testing.T) {
	b := &Backoff{
		Min:    2 * time.Second,
		Max:    100 * time.Second,
		Factor: 10,
	}

	equals(t, b.Duration(), 2*time.Second)
	equals(t, b.Duration(), 20*time.Second)
	equals(t, b.Duration(), b.Max)
	b.Reset()
	equals(t, b.Duration(), 2*time.Second)
}

func TestAttempt(t *testing.T) {
	b := &Backoff{
		Min:    2 * time.Second,
		Max:    100 * time.Second,
		Factor: 10,
	}

	equals(t, b.Attempt(), int64(0))
	equals(t, b.Duration(), 2*time.Second)
	equals(t, b.Attempt(), int64(1))
	equals(t, b.Duration(), 20*time.Second)
	equals(t, b.Attempt(), int64(2))
	equals(t, b.Duration(), b.Max)
	equals(t, b.Attempt(), int64(3))
	b.Reset()
	equals(t, b.Attempt(), int64(0))
	equals(t, b.Duration(), 2*time.Second)
	equals(t, b.Attempt(), int64(1))
}

func TestAttemptWithZeroValue(t *testing.T) {
	b := &Backoff{}

	cur := DefaultMinBackoff
	equals(t, b.Attempt(), int64(0))
	equals(t, b.Duration(), cur)
	for i := 1; i < 10; i++ {
		equals(t, b.Attempt(), int64(i))
		cur *= DefaultBackoffFactor
		if cur >= DefaultMaxBackoff {
			equals(t, b.Duration(), DefaultMaxBackoff)
			break
		}
		equals(t, b.Duration(), cur)
	}

	b.Reset()
	equals(t, b.Attempt(), int64(0))
	equals(t, b.Duration(), DefaultMinBackoff)
	equals(t, b.Attempt(), int64(1))
}

func TestNegOrZeroMin(t *testing.T) {
	b := &Backoff{
		Min: 0,
	}
	equals(t, b.Duration(), DefaultMinBackoff)

	b2 := &Backoff{
		Min: -1 * time.Second,
	}
	equals(t, b2.Duration(), DefaultMinBackoff)
}

func TestNegOrZeroMax(t *testing.T) {
	b := &Backoff{
		Min: DefaultMaxBackoff,
		Max: 0,
	}
	equals(t, b.Duration(), DefaultMaxBackoff)

	b2 := &Backoff{
		Min: DefaultMaxBackoff,
		Max: -1 * time.Second,
	}
	equals(t, b2.Duration(), DefaultMaxBackoff)
}

func TestMinLargerThanMax(t *testing.T) {
	b := &Backoff{
		Min:    500 * time.Second,
		Max:    100 * time.Second,
		Factor: 1,
	}
	equals(t, b.Duration(), b.Max)
}

func TestFakeCur(t *testing.T) {
	b := &Backoff{
		Min:    100 * time.Second,
		Max:    500 * time.Second,
		Factor: 1,
		cur:    1000 * time.Second,
	}
	equals(t, b.Duration(), b.Max)
}

func TestLargeFactor(t *testing.T) {
	b := &Backoff{
		Min:    1 * time.Second,
		Max:    10 * time.Second,
		Factor: math.MaxFloat64,
	}
	equals(t, b.Duration(), b.Min)
	for i := 0; i < 100; i++ {
		equals(t, b.Duration(), b.Max)
	}
	b.Reset()
	equals(t, b.Duration(), b.Min)
}

func TestLargeMinMax(t *testing.T) {
	b := &Backoff{
		Min:    time.Duration(math.MaxInt64 - 1),
		Max:    time.Duration(math.MaxInt64),
		Factor: math.MaxFloat64,
	}
	equals(t, b.Duration(), b.Min)
	for i := 0; i < 100; i++ {
		equals(t, b.Duration(), b.Max)
	}
	b.Reset()
	equals(t, b.Duration(), b.Min)
}

func TestManyAttempts(t *testing.T) {
	b := &Backoff{
		Min:    10 * time.Second,
		Max:    time.Duration(math.MaxInt64),
		Factor: 1000,
	}
	for i := 0; i < 10000; i++ {
		b.Duration()
	}
	equals(t, b.Duration(), b.Max)
	b.Reset()
}

func equals(t *testing.T, v1, v2 interface{}) {
	if !reflect.DeepEqual(v1, v2) {
		assert.Failf(t, "not equal", "Got %v, Expecting %v", v1, v2)
	}
}
