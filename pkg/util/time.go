package util

import (
	"fmt"
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

func Over24Hours(since time.Time) bool {
	return time.Since(since) >= 24 * time.Hour
}

func UnixMilli() int64 {
	return time.Now().UnixNano() / int64(time.Millisecond)
}

func ParseTimeWithFormats(strTime string, formats []string) (time.Time, error) {
	for _, format := range formats {
		tt, err := time.Parse(format, strTime)
		if err == nil {
			return tt, nil
		}
	}
	return time.Time{}, fmt.Errorf("failed to parse time %s, valid formats are %+v", strTime, formats)
}




