package types

// BollingerSetting contains the bollinger indicator setting propers
// Interval, Window and BandWidth
type BollingerSetting struct {
	IntervalWindow
	BandWidth float64 `json:"bandWidth"`
}
