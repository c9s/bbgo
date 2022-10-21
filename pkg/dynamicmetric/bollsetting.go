package dynamicmetric

import "github.com/c9s/bbgo/pkg/types"

// BollingerSetting is for Bollinger Band settings
type BollingerSetting struct {
	types.IntervalWindow
	BandWidth float64 `json:"bandWidth"`
}
