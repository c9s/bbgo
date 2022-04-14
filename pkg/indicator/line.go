package indicator

import (
	"time"

	"github.com/c9s/bbgo/pkg/types"
)

// Line indicator is a utility that helps to simulate either the
// 1. trend
// 2. support
// 3. resistance
// of the market data, defined with series interface
type Line struct {
	types.IntervalWindow
	start       float64
	end         float64
	startIndex  int
	endIndex    int
	currentTime time.Time
	Interval    types.Interval
}

func (l *Line) handleKLineWindowUpdate(interval types.Interval, window types.KLineWindow) {
	if interval != l.Interval {
		return
	}
	newTime := window.Last().EndTime.Time()
	delta := int(newTime.Sub(l.currentTime).Minutes()) / l.Interval.Minutes()
	l.startIndex += delta
	l.endIndex += delta
	l.currentTime = newTime
}

func (l *Line) Bind(updater KLineWindowUpdater) {
	updater.OnKLineWindowUpdate(l.handleKLineWindowUpdate)
}

func (l *Line) Last() float64 {
	return (l.end-l.start) / float64(l.startIndex - l.endIndex) * float64(l.endIndex) + l.end
}

func (l *Line) Index(i int) float64 {
	return (l.end-l.start) / float64(l.startIndex - l.endIndex) * float64(l.endIndex - i) + l.end
}

func (l *Line) Length() int {
	if l.startIndex > l.endIndex {
		return l.startIndex - l.endIndex
	} else {
		return l.endIndex - l.startIndex
	}
}

func (l *Line) SetXY1(index int, value float64) {
	l.startIndex = index
	l.start = value
}

func (l *Line) SetXY2(index int, value float64) {
	l.endIndex = index
	l.end = value
}

func NewLine(startIndex int, startValue float64, endIndex int, endValue float64, interval types.Interval) *Line {
	return &Line{
		start:       startValue,
		end:         endValue,
		startIndex:  startIndex,
		endIndex:    endIndex,
		currentTime: time.Time{},
		Interval:    interval,
	}
}

var _ types.Series = &Line{}
