package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPriceVolumeSlice_Remove(t *testing.T) {
	for _, descending := range []bool{true, false} {
		slice := PriceVolumeSlice{}
		slice = slice.Upsert(PriceVolume{Price: 1}, descending)
		slice = slice.Upsert(PriceVolume{Price: 3}, descending)
		slice = slice.Upsert(PriceVolume{Price: 5}, descending)
		assert.Equal(t, 3, len(slice), "with descending %v", descending)

		slice = slice.Remove(2, descending)
		assert.Equal(t, 3, len(slice), "with descending %v", descending)

		slice = slice.Remove(3, descending)
		assert.Equal(t, 2, len(slice), "with descending %v", descending)

		slice = slice.Remove(99, descending)
		assert.Equal(t, 2, len(slice), "with descending %v", descending)

		slice = slice.Remove(0, descending)
		assert.Equal(t, 2, len(slice), "with descending %v", descending)
	}
}
