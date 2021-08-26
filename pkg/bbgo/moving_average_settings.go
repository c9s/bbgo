package bbgo

import (
	"fmt"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/strategy/schedule"
	"github.com/c9s/bbgo/pkg/types"
)

type MovingAverageSettings struct {
	Type     string         `json:"type"`
	Interval types.Interval `json:"interval"`
	Window   int            `json:"window"`

	Side     *types.SideType   `json:"side"`
	Quantity *fixedpoint.Value `json:"quantity"`
	Amount   *fixedpoint.Value `json:"amount"`
}

func (settings MovingAverageSettings) IntervalWindow() types.IntervalWindow {
	var window = 99
	if settings.Window > 0 {
		window = settings.Window
	}

	return types.IntervalWindow{
		Interval: settings.Interval,
		Window:   window,
	}
}

func (settings *MovingAverageSettings) Indicator(indicatorSet *StandardIndicatorSet) (inc schedule.Float64Indicator, err error) {
	var iw = settings.IntervalWindow()

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
