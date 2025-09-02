//go:build !dnum

package fixedpoint

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestExpirableValue_SetAndGet(t *testing.T) {
	v := &ExpirableValue{}
	val := Value(42)
	expires := time.Unix(1000000000, 0)
	now := expires.Add(-time.Second)
	assert.NoError(t, v.Set(val, expires))

	got, ok := v.Get(now)
	assert.Equal(t, val, got, "Expected value should match")
	assert.True(t, ok, "Should not be expired")
}

func TestExpirableValue_IsExpired(t *testing.T) {
	v := &ExpirableValue{}
	val := Value(42)
	expires := time.Now().Add(-1 * time.Hour)
	assert.NoError(t, v.Set(val, expires))
	assert.True(t, v.IsExpired(time.Now()), "Should be expired")
}

func TestExpirableValue_GetExpired(t *testing.T) {
	val := Value(42)
	expires := time.Unix(1000000000, 0)
	v := NewExpirable(val, expires)
	now := expires.Add(time.Second)
	assert.NoError(t, v.Set(val, expires))
	got, ok := v.Get(now)
	assert.Equal(t, Zero, got, "Expired value should be Zero")
	assert.False(t, ok, "Should be expired")
}

func TestExpirableValue_GetUnset(t *testing.T) {
	v := &ExpirableValue{}
	got, ok := v.Get(time.Now())
	assert.Equal(t, Zero, got, "Unset value should be Zero")
	assert.False(t, ok, "Unset value should not be expired")
}

func TestExpirableValue_String(t *testing.T) {
	val := Value(42)
	expires := time.Date(2025, 9, 2, 12, 0, 0, 0, time.UTC)
	v := NewExpirable(val, expires)
	assert.NoError(t, v.Set(val, expires))
	str := v.String()
	assert.NotEmpty(t, str, "String output should not be empty")
	assert.NotEqual(t, "42", str, "String output should include expires at info")
	assert.Contains(t, str, "expires at 2025-09-02T12:00:00Z", "Expected string to contain expires at info")
}
