package factorzoo

import (
	"fmt"
	"time"

	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/types"

	log "github.com/sirupsen/logrus"
)

//go:generate callbackgen -type LSBAR
type LSBAR struct {
	types.SeriesBase
	types.IntervalWindow

	// Values
	Values    types.Float64Slice
	LastValue float64

	EndTime time.Time

	UpdateCallbacks []func(val float64)
}

func (inc *LSBAR) Index(i int) float64 {
	if inc.Values == nil {
		return 0
	}
	return inc.Values.Index(i)
}

func (inc *LSBAR) Last() float64 {
	if inc.Values.Length() == 0 {
		return 0
	}
	return inc.Values.Last()
}

func (inc *LSBAR) Length() int {
	if inc.Values == nil {
		return 0
	}
	return inc.Values.Length()
}

//var _ types.SeriesExtend = &LSBAR{}

func (inc *LSBAR) Update(klines []types.KLine) {
	if inc.Values == nil {
		inc.SeriesBase.Series = inc
	}

	if len(klines) < inc.Window {
		return
	}

	var end = len(klines) - 1
	var lastKLine = klines[end]

	if inc.EndTime != zeroTime && lastKLine.GetEndTime().Before(inc.EndTime) {
		return
	}

	var recentT = klines[end-(inc.Window-1) : end+1]

	val, err := calculateBar(recentT, inc.Window, indicator.KLineOpenPriceMapper, indicator.KLineClosePriceMapper, indicator.KLineHighPriceMapper, indicator.KLineLowPriceMapper)
	if err != nil {
		log.WithError(err).Error("can not calculate")
		return
	}
	inc.Values.Push(val)
	inc.LastValue = val

	if len(inc.Values) > indicator.MaxNumOfVOL {
		inc.Values = inc.Values[indicator.MaxNumOfVOLTruncateSize-1:]
	}

	inc.EndTime = klines[end].GetEndTime().Time()

	inc.EmitUpdate(val)

}

func (inc *LSBAR) handleKLineWindowUpdate(interval types.Interval, window types.KLineWindow) {
	if inc.Interval != interval {
		return
	}

	inc.Update(window)
}

func (inc *LSBAR) Bind(updater indicator.KLineWindowUpdater) {
	updater.OnKLineWindowUpdate(inc.handleKLineWindowUpdate)
}

func calculateBar(klines []types.KLine, window int, valO KLineValueMapper, valC KLineValueMapper, valH KLineValueMapper, valL KLineValueMapper) (float64, error) {
	length := len(klines)
	if length == 0 || length < window {
		return 0.0, fmt.Errorf("insufficient elements for calculating VOL with window = %d", window)
	}

	s1 := 0.
	s2 := 0.
	for _, p := range klines[length-int(window/12) : length-1] {
		bar := (valO(p) - valC(p)) / (valH(p) - valL(p))
		if valO(p) > valC(p) {
			s1 += bar
		} else if valO(p) < valC(p) {
			s2 += -bar
		}
	}
	s := s1 / s2

	l1 := 0.
	l2 := 0.
	for _, p := range klines[length-window : length-1] {
		bar := (valO(p) - valC(p)) / (valH(p) - valL(p))
		if valO(p) > valC(p) {
			l1 += bar
		} else if valO(p) < valC(p) {
			l2 += -bar
		}
	}
	l := l1 / l2

	alpha := s / l

	return alpha, nil
}
