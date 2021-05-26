package util

import (
	"math/rand"
	"time"
)

func MillisecondsJitter(d time.Duration, jitterInMilliseconds int) time.Duration {
	n := rand.Intn(jitterInMilliseconds)
	return d + time.Duration(n)*time.Millisecond
}
