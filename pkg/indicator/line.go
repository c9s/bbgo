package indicator

import (
	"time"

	"github.com/c9s/bbgo/pkg/types"
)

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

var _ types.Series = &Line{}
