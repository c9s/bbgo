package fixedpoint

import (
	"bytes"
	"sync"
	"time"
)

type ExpirableValue struct {
	value     Value
	expiresAt time.Time
	mu        sync.Mutex
}

func NewExpirable(value Value, expiresAt time.Time) *ExpirableValue {
	return &ExpirableValue{
		value:     value,
		expiresAt: expiresAt,
	}
}

func (v *ExpirableValue) IsExpired(now time.Time) bool {
	v.mu.Lock()
	defer v.mu.Unlock()
	return now.After(v.expiresAt)
}

func (v *ExpirableValue) String() string {
	v.mu.Lock()
	defer v.mu.Unlock()

	var buf bytes.Buffer
	buf.WriteString(v.value.String())
	if !v.expiresAt.IsZero() {
		buf.WriteString(" (expires at ")
		buf.WriteString(v.expiresAt.Format(time.RFC3339))
		buf.WriteString(")")
	}
	return buf.String()
}

func (v *ExpirableValue) Set(value Value, expiresAt time.Time) {
	v.mu.Lock()
	v.value = value
	v.expiresAt = expiresAt
	v.mu.Unlock()
}

func (v *ExpirableValue) Get(now time.Time) (Value, bool) {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.expiresAt.IsZero() {
		return Zero, false
	}

	if now.After(v.expiresAt) {
		return Zero, false
	}

	return v.value, true
}
