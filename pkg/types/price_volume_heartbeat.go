package types

import (
	"fmt"
	"time"
)

// PriceHeartBeat is used for monitoring the price volume update.
type PriceHeartBeat struct {
	PriceVolume PriceVolume
	LastTime    time.Time
}

// Update updates the price volume object and the last update time
// It returns (bool, error), when the price is successfully updated, it returns true.
// If the price is not updated (same price) and the last time exceeded the timeout,
// Then false, and an error will be returned
func (b *PriceHeartBeat) Update(pv PriceVolume, timeout time.Duration) (bool, error) {
	if b.PriceVolume.Price == 0 || b.PriceVolume != pv {
		b.PriceVolume = pv
		b.LastTime = time.Now()
		return true, nil // successfully updated
	} else if time.Since(b.LastTime) > timeout {
		return false, fmt.Errorf("price %s has not been updating for %s, last update: %s, skip quoting",
			b.PriceVolume.String(),
			time.Since(b.LastTime),
			b.LastTime)
	}

	return false, nil
}
