package csvsource

import (
	"encoding/csv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	. "github.com/c9s/bbgo/pkg/testing/testhelper"
	"github.com/c9s/bbgo/pkg/types"
)

func TestCSVTickReader_ReadWithBinanceDecoder(t *testing.T) {
	tests := []struct {
		name string
		give string
		want *CsvTick
		err  error
	}{
		{
			name: "Read Tick",
			give: "11782578,6.00000000,1.00000000,14974844,14974844,1698623884463,True,True",
			want: &CsvTick{
				Timestamp:    types.NewMillisecondTimestampFromInt(1698623884463),
				Size:         Number(1),
				Price:        Number(6),
				HomeNotional: Number(6),
			},
			err: nil,
		},
		{
			name: "Not enough columns",
			give: "1609459200000,28923.63000000,29031.34000000",
			want: nil,
			err:  ErrNotEnoughColumns,
		},
		{
			name: "Invalid time format",
			give: "11782578,6.00000000,1.00000000,14974844,14974844,23/12/2021,True,True",
			want: nil,
			err:  ErrInvalidTimeFormat,
		},
		{
			name: "Invalid price format",
			give: "11782578,sixty,1.00000000,14974844,14974844,1698623884463,True,True",
			want: nil,
			err:  ErrInvalidPriceFormat,
		},
		{
			name: "Invalid size format",
			give: "11782578,1.00000000,one,14974844,14974844,1698623884463,True,True",
			want: nil,
			err:  ErrInvalidVolumeFormat,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := NewBinanceCSVTickReader(csv.NewReader(strings.NewReader(tt.give)))
			tick, err := reader.Read(0)
			if err == nil {
				assertTickEqual(t, tt.want, tick)
			}
			assert.Equal(t, tt.err, err)
		})
	}
}

func assertTickEqual(t *testing.T, exp, act *CsvTick) {
	assert.Equal(t, exp.Timestamp.Time(), act.Timestamp.Time())
	assert.Equal(t, 0, exp.Price.Compare(act.Price))
	assert.Equal(t, 0, exp.Size.Compare(act.Size))
	assert.Equal(t, 0, exp.HomeNotional.Compare(act.HomeNotional))
}
