package util

import "time"

type VolatileMemory struct {
	objectTimes map[interface{}]time.Time
	textTimes   map[string]time.Time
}

func NewDetectorCache() *VolatileMemory {
	return &VolatileMemory{
		objectTimes: make(map[interface{}]time.Time),
		textTimes:   make(map[string]time.Time),
	}
}

func (i *VolatileMemory) IsObjectFresh(obj interface{}, ttl time.Duration) bool {
	now := time.Now()
	outdatedTime := now.Add(-ttl)

	if hitTime, ok := i.objectTimes[obj]; ok {
		if hitTime.Before(outdatedTime) {
			return true
		}
	}

	i.objectTimes[obj] = now
	return false
}

func (i *VolatileMemory) IsTextFresh(text string, ttl time.Duration) bool {
	now := time.Now()
	outdatedTime := now.Add(-ttl)

	if hitTime, ok := i.textTimes[text]; ok {
		if hitTime.Before(outdatedTime) {
			return true
		}
	}

	i.textTimes[text] = now
	return false
}
