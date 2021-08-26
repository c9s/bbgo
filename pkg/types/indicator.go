package types

// Float64Indicator is the indicators (SMA and EWMA) that we want to use are returning float64 data.
type Float64Indicator interface {
	Last() float64
}
