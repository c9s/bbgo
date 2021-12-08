package types

import (
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

type FundingRate struct {
	FundingRate fixedpoint.Value
	FundingTime time.Time
	Time        time.Time
}
