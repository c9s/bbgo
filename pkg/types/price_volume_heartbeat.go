package types

import (
	"fmt"
	"time"
)

type PriceHeartBeat struct {
	PriceVolume PriceVolume
	LastTime    time.Time
}

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
