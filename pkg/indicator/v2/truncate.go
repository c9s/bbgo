package indicatorv2

// MaxSliceSize is the maximum slice size
// byte size = 8 * 5000 = 40KB per slice
const MaxSliceSize = 5000

// TruncateSize is the truncate size for the slice per truncate call
const TruncateSize = 1000

func generalTruncate(slice []float64) []float64 {
	if len(slice) < MaxSliceSize {
		return slice
	}

	return slice[TruncateSize-1:]
}
