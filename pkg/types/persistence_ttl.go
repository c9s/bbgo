package types

import "time"

type PersistenceTTL struct {
	ttl time.Duration
}

func (p *PersistenceTTL) SetTTL(ttl time.Duration) {
	if ttl.Nanoseconds() <= 0 {
		return
	}
	p.ttl = ttl
}

func (p *PersistenceTTL) Expiration() time.Duration {
	return p.ttl
}
