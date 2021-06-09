package util

import "time"

type TimeProfile struct {
	StartTime, EndTime time.Time
	Duration           time.Duration
}

func StartTimeProfile() TimeProfile {
	return TimeProfile{StartTime: time.Now()}
}

func (p *TimeProfile) TilNow() time.Duration {
	return time.Now().Sub(p.StartTime)
}

func (p *TimeProfile) Stop() time.Duration {
	p.EndTime = time.Now()
	p.Duration = p.EndTime.Sub(p.StartTime)
	return p.Duration
}
