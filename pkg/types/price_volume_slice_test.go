package types

import (
	"testing"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/stretchr/testify/assert"
)

func TestPriceVolumeSlice_Remove(t *testing.T) {
	for _, descending := range []bool{true, false} {
		slice := PriceVolumeSlice{}
		slice = slice.Upsert(PriceVolume{Price: fixedpoint.One}, descending)
		slice = slice.Upsert(PriceVolume{Price: fixedpoint.NewFromInt(3)}, descending)
		slice = slice.Upsert(PriceVolume{Price: fixedpoint.NewFromInt(5)}, descending)
		assert.Equal(t, 3, len(slice), "with descending %v", descending)

		slice = slice.Remove(fixedpoint.NewFromInt(2), descending)
		assert.Equal(t, 3, len(slice), "with descending %v", descending)

		slice = slice.Remove(fixedpoint.NewFromInt(3), descending)
		assert.Equal(t, 2, len(slice), "with descending %v", descending)

		slice = slice.Remove(fixedpoint.NewFromInt(99), descending)
		assert.Equal(t, 2, len(slice), "with descending %v", descending)

		slice = slice.Remove(fixedpoint.Zero, descending)
		assert.Equal(t, 2, len(slice), "with descending %v", descending)
	}
}
