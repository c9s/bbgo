package indicatorv2

import "github.com/c9s/bbgo/pkg/types"

// MaxSliceSize is the maximum slice size
// byte size = 8 * 5000 = 40KB per slice
const MaxSliceSize = 5000

// TruncateSize is the truncate size for the slice per truncate call
const TruncateSize = 1000

func generalTruncate(slice []float64) []float64 {
	return types.ShrinkSlice(slice, MaxSliceSize, TruncateSize)
}
