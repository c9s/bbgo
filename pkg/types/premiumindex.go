package types

import (
	"fmt"
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

type PremiumIndex struct {
	Symbol          string           `json:"symbol"`
	MarkPrice       fixedpoint.Value `json:"markPrice"`
	LastFundingRate fixedpoint.Value `json:"lastFundingRate"`
	NextFundingTime time.Time        `json:"nextFundingTime"`
	Time            time.Time        `json:"time"`
}

func (i *PremiumIndex) String() string {
	return fmt.Sprintf("PremiumIndex | %s | %.4f | %s | %s | NEXT FUNDING TIME: %s", i.Symbol, i.MarkPrice.Float64(), i.LastFundingRate.Percentage(), i.Time, i.NextFundingTime)
}
