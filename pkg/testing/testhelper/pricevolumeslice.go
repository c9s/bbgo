package testhelper

import (
	"fmt"
	"strings"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func PriceVolumeSliceFromText(str string) (slice types.PriceVolumeSlice) {
	lines := strings.Split(str, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if len(line) == 0 {
			continue
		}

		cols := strings.SplitN(line, ",", 2)
		if len(cols) < 2 {
			panic(fmt.Errorf("column length should be 2, got %d", len(cols)))
		}

		price := fixedpoint.MustNewFromString(strings.TrimSpace(cols[0]))
		volume := fixedpoint.MustNewFromString(strings.TrimSpace(cols[1]))
		slice = append(slice, types.PriceVolume{
			Price:  price,
			Volume: volume,
		})
	}

	return slice
}

func PriceVolumeSlice(values ...fixedpoint.Value) (slice types.PriceVolumeSlice) {
	if len(values)%2 != 0 {
		panic("values should be paired")
	}

	for i := 0; i < len(values); i += 2 {
		slice = append(slice, types.PriceVolume{
			Price:  values[i],
			Volume: values[i+1],
		})

	}

	return slice
}
