package timejitter

import (
	"math/rand"
	"time"
)

func Milliseconds(d time.Duration, jitterInMilliseconds int) time.Duration {
	n := rand.Intn(jitterInMilliseconds)
	return d + time.Duration(n)*time.Millisecond
}

func Seconds(d time.Duration, jitterInSeconds int) time.Duration {
	n := rand.Intn(jitterInSeconds)
	return d + time.Duration(n)*time.Second
}

func Microseconds(d time.Duration, us int) time.Duration {
	n := rand.Intn(us)
	return d + time.Duration(n)*time.Microsecond
}
