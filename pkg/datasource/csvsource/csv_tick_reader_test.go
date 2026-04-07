//go:build !dnum

package csvsource

import (
	"encoding/csv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	. "github.com/c9s/bbgo/pkg/testing/testhelper"
	"github.com/c9s/bbgo/pkg/types"
)

func TestCSVTickReader_Read(t *testing.T) {
	tests := []struct {
		name string
		give string
		want *CsvTick
		err  error
	}{
		{
			name: "Read buy tick",
			give: "11782578,buy,1.00000000,6.00000000,1698623884463",
			want: &CsvTick{
				TradeID:      11782578,
				Side:         types.SideTypeBuy,
				Timestamp:    types.NewMillisecondTimestampFromInt(1698623884463),
				Size:         Number(1),
				Price:        Number(6),
				HomeNotional: Number(1),
			},
			err: nil,
		},
		{
			name: "Read sell tick",
			give: "100,sell,2.50000000,10.00000000,1698623884463",
			want: &CsvTick{
				TradeID:      100,
				Side:         types.SideTypeSell,
				Timestamp:    types.NewMillisecondTimestampFromInt(1698623884463),
				Size:         Number(2.5),
				Price:        Number(10),
				HomeNotional: Number(2.5),
			},
			err: nil,
		},
		{
			name: "Not enough columns",
			give: "11782578,buy,1.00000000",
			want: nil,
			err:  ErrNotEnoughColumns,
		},
		{
			name: "Invalid time format",
			give: "11782578,buy,1.00000000,6.00000000,23/12/2021",
			want: nil,
			err:  ErrInvalidTimeFormat,
		},
		{
			name: "Invalid price format",
			give: "11782578,buy,1.00000000,6o,1698623884463",
			want: nil,
			err:  ErrInvalidPriceFormat,
		},
		{
			name: "Invalid size format",
			give: "11782578,buy,one,6.00000000,1698623884463",
			want: nil,
			err:  ErrInvalidVolumeFormat,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := NewCSVTickReader(csv.NewReader(strings.NewReader(tt.give)))
			tick, err := reader.Read()
			if err == nil {
				assertTickEqual(t, tt.want, tick)
			}
			assert.Equal(t, tt.err, err)
		})
	}
}

func assertTickEqual(t *testing.T, exp, act *CsvTick) {
	assert.Equal(t, exp.TradeID, act.TradeID)
	assert.Equal(t, exp.Side, act.Side)
	assert.Equal(t, exp.Timestamp.Time(), act.Timestamp.Time())
	assert.Equal(t, 0, exp.Price.Compare(act.Price))
	assert.Equal(t, 0, exp.Size.Compare(act.Size))
	assert.Equal(t, 0, exp.HomeNotional.Compare(act.HomeNotional))
}
