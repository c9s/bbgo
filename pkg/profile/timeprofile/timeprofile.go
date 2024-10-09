package timeprofile

import (
	"time"
)

type logFunction func(format string, args ...interface{})

type TimeProfile struct {
	Name               string
	StartTime, EndTime time.Time
	Duration           time.Duration
}

func Start(args ...string) TimeProfile {
	name := ""
	if len(args) > 0 {
		name = args[0]
	}

	return TimeProfile{StartTime: time.Now(), Name: name}
}

// TilNow returns the duration from the start time to now
func (p *TimeProfile) TilNow() time.Duration {
	return time.Since(p.StartTime)
}

// Stop stops the time profile, set the end time and returns the duration
func (p *TimeProfile) Stop() time.Duration {
	p.EndTime = time.Now()
	p.Duration = p.EndTime.Sub(p.StartTime)
	return p.Duration
}

// Do runs the function f and stops the time profile
func (p *TimeProfile) Do(f func()) {
	defer p.Stop()
	f()
}

func (p *TimeProfile) StopAndLog(f logFunction) {
	duration := p.Stop()
	s := "[profile] "
	if len(p.Name) > 0 {
		s += p.Name
	}

	s += " " + duration.String()
	f(s)
}
