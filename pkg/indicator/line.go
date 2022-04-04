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
	startTime   time.Time
	endTime     time.Time
	currentTime time.Time
	Interval    types.Interval
}

func (l *Line) handleKLineWindowUpdate(interval types.Interval, window types.KLineWindow) {
	if interval != l.Interval {
		return
	}
	l.currentTime = window.Last().EndTime.Time()
}

func (l *Line) Bind(updater KLineWindowUpdater) {
	updater.OnKLineWindowUpdate(l.handleKLineWindowUpdate)
}

func (l *Line) Last() float64 {
	return l.currentTime.Sub(l.endTime).Minutes()*(l.end-l.start)/l.endTime.Sub(l.startTime).Minutes() + l.end
}

func (l *Line) Index(i int) float64 {
	return (l.currentTime.Sub(l.endTime).Minutes()-float64(i*l.Interval.Minutes()))*(l.end-l.start)/l.endTime.Sub(l.startTime).Minutes() + l.end
}

func (l *Line) Length() int {
	return int(l.startTime.Sub(l.currentTime).Minutes()) / l.Interval.Minutes()
}

func (l *Line) SetX1(value float64, startTime time.Time) {
	l.startTime = startTime
	l.start = value
}

func (l *Line) SetX2(value float64, endTime time.Time) {
	l.endTime = endTime
	l.end = value
}

func NewLine(startValue float64, startTime time.Time, endValue float64, endTime time.Time, interval types.Interval) *Line {
	return &Line{
		start:       startValue,
		end:         endValue,
		startTime:   startTime,
		endTime:     endTime,
		currentTime: endTime,
		Interval:    interval,
	}
}

var _ types.Series = &Line{}
