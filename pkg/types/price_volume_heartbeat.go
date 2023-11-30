package types

import (
	"fmt"
	"time"
)

// PriceHeartBeat is used for monitoring the price volume update.
type PriceHeartBeat struct {
	last            PriceVolume
	lastUpdatedTime time.Time
	timeout         time.Duration
}

func NewPriceHeartBeat(timeout time.Duration) *PriceHeartBeat {
	return &PriceHeartBeat{
		timeout: timeout,
	}
}

func (b *PriceHeartBeat) Last() PriceVolume {
	return b.last
}

// Update updates the price volume object and the last update time
// It returns (bool, error), when the price is successfully updated, it returns true.
// If the price is not updated (same price) and the last time exceeded the timeout,
// Then false, and an error will be returned
func (b *PriceHeartBeat) Update(current PriceVolume) (bool, error) {
	if b.last.Price.IsZero() || b.last != current {
		b.last = current
		b.lastUpdatedTime = time.Now()
		return true, nil // successfully updated
	} else {
		// if price and volume is not changed
		if b.last.Equals(current) && time.Since(b.lastUpdatedTime) > b.timeout {
			return false, fmt.Errorf("price %s has not been updating for %s, last update: %s, skip quoting",
				b.last.String(),
				time.Since(b.lastUpdatedTime),
				b.lastUpdatedTime)
		}
	}

	return false, nil
}
