package bbgo

import (
	"time"
)

var LocalTimeZone *time.Location

func init() {
	var err error
	LocalTimeZone, err = time.LoadLocation("Local")
	if err != nil {
		panic(err)
	}
}
