package util

import (
	"time"

	"github.com/c9s/bbgo/pkg/types"
	log "github.com/sirupsen/logrus"
)

type DailyDataTracker struct {
	StartedAt time.Time      `json:"startedAt,omitempty"`
	Data      types.ValueMap `json:"data,omitempty"`
}

func (d *DailyDataTracker) IsOver24Hours() bool {
	return time.Since(d.StartedAt) >= 24*time.Hour
}

func (d *DailyDataTracker) ResetTime() {
	t := time.Now()
	dateTime := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())

	log.Infof("[Countdown] resetting accumulated started time to: %s", dateTime)

	d.StartedAt = dateTime
}

func (d *DailyDataTracker) ResetData() {
	d.Data = make(types.ValueMap)
}

func (d *DailyDataTracker) Reset() {
	d.ResetTime()
	d.ResetData()
}
