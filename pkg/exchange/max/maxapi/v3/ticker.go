package v3

import (
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

type Ticker struct {
	Time time.Time

	At          int64            `json:"at"`
	Buy         fixedpoint.Value `json:"buy"`
	Sell        fixedpoint.Value `json:"sell"`
	Open        fixedpoint.Value `json:"open"`
	High        fixedpoint.Value `json:"high"`
	Low         fixedpoint.Value `json:"low"`
	Last        fixedpoint.Value `json:"last"`
	Volume      fixedpoint.Value `json:"vol"`
	VolumeInBTC fixedpoint.Value `json:"vol_in_btc"`
}
