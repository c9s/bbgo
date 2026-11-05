package bbgo

import (
	"fmt"

	"github.com/c9s/bbgo/pkg/types"
)

type MovingAverageSettings struct {
	Type     string         `json:"type"`
	Interval types.Interval `json:"interval"`
	Window   int            `json:"window"`

	Side *types.SideType `json:"side"`

	QuantityOrAmount
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

func (settings *MovingAverageSettings) Indicator(indicatorSet *StandardIndicatorSet) (inc types.Float64Indicator, err error) {
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

// IndicatorV2 returns the v2 moving average indicator from the given indicator set
func (settings *MovingAverageSettings) IndicatorV2(indicatorSet *IndicatorSet) (types.Float64Indicator, error) {
	var iw = settings.IntervalWindow()

	switch settings.Type {
	case "SMA":
		return indicatorSet.SMA(iw), nil

	case "EWMA", "EMA":
		return indicatorSet.EWMA(iw), nil

	default:
		return nil, fmt.Errorf("unsupported moving average type: %s", settings.Type)
	}
}
