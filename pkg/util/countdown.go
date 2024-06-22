package util

import (
	"time"

	log "github.com/sirupsen/logrus"
)

type Countdown struct {
	StartedAt time.Time `json:"startedAt,omitempty"`
}

func (c *Countdown) IsOver24Hours() bool {
	return time.Since(c.StartedAt) >= 24*time.Hour
}

func (s *Countdown) Reset() {
	t := time.Now()
	dateTime := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())

	log.Infof("[Countdown] resetting accumulated started time to: %s", dateTime)

	s.StartedAt = dateTime
}
