package indicator

import (
	"fmt"
	"math"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/types"
)

type Float64Slice []float64

func (s *Float64Slice) Push(v float64) {
	*s = append(*s, v)
}

func (s *Float64Slice) Pop(i int64) (v float64) {
	v = (*s)[i]
	*s = append((*s)[:i], (*s)[i+1:]...)
	return v
}

func (s Float64Slice) Max() float64 {
	m := -math.MaxFloat64
	for _, v := range s {
		m = math.Max(m, v)
	}
	return m
}

func (s Float64Slice) Min() float64 {
	m := math.MaxFloat64
	for _, v := range s {
		m = math.Min(m, v)
	}
	return m
}

func (s Float64Slice) Sum() (sum float64) {
	for _, v := range s {
		sum += v
	}
	return sum
}

func (s Float64Slice) Mean() (mean float64) {
	return s.Sum() / float64(len(s))
}

func (s Float64Slice) Tail(size int) Float64Slice {
	length := len(s)
	if length <= size {
		win := make(Float64Slice, length)
		copy(win, s)
		return win
	}

	win := make(Float64Slice, size)
	copy(win, s[length-size:])
	return win
}

var zeroTime time.Time

//go:generate callbackgen -type SMA
type SMA struct {
	types.IntervalWindow
	Values  Float64Slice
	EndTime time.Time

	UpdateCallbacks []func(value float64)
}

func (inc *SMA) Last() float64 {
	return inc.Values[len(inc.Values)-1]
}

func (inc *SMA) calculateAndUpdate(kLines []types.KLine) {
	if len(kLines) < inc.Window {
		return
	}

	var index = len(kLines) - 1
	var kline = kLines[index]

	if inc.EndTime != zeroTime && kline.EndTime.Before(inc.EndTime) {
		return
	}

	var recentK = kLines[index-(inc.Window-1) : index+1]

	sma, err := calculateSMA(recentK, inc.Window, KLineClosePriceMapper)
	if err != nil {
		log.WithError(err).Error("SMA error")
		return
	}
	inc.Values.Push(sma)
	inc.EndTime = kLines[index].EndTime

	inc.EmitUpdate(sma)
}

func (inc *SMA) handleKLineWindowUpdate(interval types.Interval, window types.KLineWindow) {
	if inc.Interval != interval {
		return
	}

	inc.calculateAndUpdate(window)
}

func (inc *SMA) Bind(updater KLineWindowUpdater) {
	updater.OnKLineWindowUpdate(inc.handleKLineWindowUpdate)
}

func calculateSMA(kLines []types.KLine, window int, priceF KLinePriceMapper) (float64, error) {
	length := len(kLines)
	if length == 0 || length < window {
		return 0.0, fmt.Errorf("insufficient elements for calculating SMA with window = %d", window)
	}

	sum := 0.0
	for _, k := range kLines {
		sum += priceF(k)
	}

	avg := sum / float64(window)
	return avg, nil
}
