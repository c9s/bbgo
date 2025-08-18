package types

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

func TestPriceVolumeSlice_UnmarshalJSON(t *testing.T) {
	t.Run("array of array", func(t *testing.T) {
		input := []byte(`[["19000.0","3.0"],["19111.0","2.0"]]`)
		slice, err := ParsePriceVolumeSliceJSON(input)
		if assert.NoError(t, err) {
			assert.Len(t, slice, 2)
			assert.Equal(t, "19000", slice[0].Price.String())
			assert.Equal(t, "3", slice[0].Volume.String())
		}
	})

	t.Run("array of object", func(t *testing.T) {
		input := []byte(`[{ "Price": "19000.0", "Volume":"3.0"},{"Price": "19111.0","Volume": "2.0" }]`)
		slice, err := ParsePriceVolumeSliceJSON(input)
		if assert.NoError(t, err) {
			assert.Len(t, slice, 2)
			assert.Equal(t, "19000", slice[0].Price.String())
			assert.Equal(t, "3", slice[0].Volume.String())
		}
	})
}

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
