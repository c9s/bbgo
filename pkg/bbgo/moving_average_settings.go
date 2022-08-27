package bbgo

import (
	"fmt"

	"github.com/c9s/bbgo/pkg/types"
)

type MovingAverageSettings struct {
	Type     string         `json:"type"`

	types.IntervalWindow

	Side *types.SideType `json:"side"`
	QuantityOrAmount
}

func (settings *MovingAverageSettings) Indicator(indicatorSet *StandardIndicatorSet) (inc types.Float64Indicator, err error) {
	var iw = settings.IntervalWindow
	if iw.Window > 0 {
		iw.Window = 99
	}

	switch settings.Type {
	case "SMA":
		inc = indicatorSet.SMA(iw)

	case "EWMA", "EMA":
		inc = indicatorSet.EWMA(iw)

	default:
		return nil, fmt.Errorf("unsupported moving average type: %s", settings.Type)
	}

	return inc, nil
}
