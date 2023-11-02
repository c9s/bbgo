package indicatorv2

import (
	"encoding/json"
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func Test_Drift(t *testing.T) {
	var randomPrices = []byte(`[1, 1, 2, 3, 4, 5, 6, 7, 8, 9, 4, 1, 2, 3, 4, 5, 6, 7, 8, 9, 4, 1, 2, 3, 4, 5, 6, 7, 8, 9, 1, 1, 2, 3, 4, 5, 6, 7, 8, 9, 4, 1, 2, 3, 4, 5, 6, 7, 8, 9]`)
	var input []fixedpoint.Value
	_ = json.Unmarshal(randomPrices, &input)

	tests := []struct {
		name   string
		kLines []types.KLine
		all    int
	}{
		{
			name:   "random_case",
			kLines: buildKLines(input),
			all:    47,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stream := &types.StandardStream{}
			kLines := KLines(stream, "", "")
			ind := Drift(kLines, 3)
			for _, candle := range tt.kLines {
				stream.EmitKLineClosed(candle)
			}
			spew.Dump(ind)
			assert.Equal(t, ind.Length(), tt.all)
			for _, v := range ind.Slice {
				assert.LessOrEqual(t, v, 1.0)
			}
		})
	}
}
