package util

import (
	"time"
)

type TimeProfile struct {
	Name               string
	StartTime, EndTime time.Time
	Duration           time.Duration
}

func StartTimeProfile(args ...string) TimeProfile {
	name := ""
	if len(args) > 0 {
		name = args[0]
	}
	return TimeProfile{StartTime: time.Now(), Name: name}
}

func (p *TimeProfile) TilNow() time.Duration {
	return time.Since(p.StartTime)
}

func (p *TimeProfile) Stop() time.Duration {
	p.EndTime = time.Now()
	p.Duration = p.EndTime.Sub(p.StartTime)
	return p.Duration
}

type logFunction func(format string, args ...interface{})

func (p *TimeProfile) StopAndLog(f logFunction) {
	duration := p.Stop()
	s := "[profile] "
	if len(p.Name) > 0 {
		s += p.Name
	}

	s += " " + duration.String()
	f(s)
}
