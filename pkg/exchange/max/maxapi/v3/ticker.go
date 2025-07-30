package v3

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type Ticker struct {
	Market string `json:"market"`

	At types.MillisecondTimestamp `json:"at"`

	Buy    fixedpoint.Value `json:"buy"`
	BuyVol fixedpoint.Value `json:"buy_vol"`

	Sell    fixedpoint.Value `json:"sell"`
	SellVol fixedpoint.Value `json:"sell_vol"`

	Open fixedpoint.Value `json:"open"`
	High fixedpoint.Value `json:"high"`
	Low  fixedpoint.Value `json:"low"`
	Last fixedpoint.Value `json:"last"`

	Volume        fixedpoint.Value `json:"vol"`
	VolumeInBTC   fixedpoint.Value `json:"vol_in_btc"`
	VolumeInQuote fixedpoint.Value `json:"vol_in_quote"`
}
