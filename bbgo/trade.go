package bbgo

import (
	"time"
)

type Trade struct {
	ID          int64
	Price       float64
	Volume      float64
	IsBuyer     bool
	IsMaker     bool
	Time        time.Time
	Market      string
	Fee         float64
	FeeCurrency string
}

