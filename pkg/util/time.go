package util

import (
	"math/rand"
	"time"
)

func MillisecondsJitter(d time.Duration, jitterInMilliseconds int) time.Duration {
	n := rand.Intn(jitterInMilliseconds)
	return d + time.Duration(n)*time.Millisecond
}

func BeginningOfTheDay(t time.Time) time.Time {
	year, month, day := t.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, t.Location())
}

