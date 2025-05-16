package apikey

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRoundTripBalancer(t *testing.T) {
	// Prepare test data
	entries := []Entry{
		{Index: 1, Key: "key1", Secret: "secret1"},
		{Index: 2, Key: "key2", Secret: "secret2"},
		{Index: 3, Key: "key3", Secret: "secret3"},
	}
	source := &Source{Entries: entries}

	// Initialize RoundTripBalancer
	balancer := NewRoundTripBalancer(source)

	// Test Next method
	assert.Equal(t, &entries[0], balancer.Next(), "First call to Next should return the first entry")
	assert.Equal(t, &entries[1], balancer.Next(), "Second call to Next should return the second entry")
	assert.Equal(t, &entries[2], balancer.Next(), "Third call to Next should return the third entry")
	assert.Equal(t, &entries[0], balancer.Next(), "Fourth call to Next should wrap around to the first entry")

	// Test Peek method
	assert.Equal(t, &entries[1], balancer.Peek(), "Peek should return the next entry without advancing the position")
	assert.Equal(t, &entries[1], balancer.Next(), "Next should still return the next entry after Peek")

	// Test Reset method
	balancer.Reset()
	assert.Equal(t, &entries[0], balancer.Next(), "After Reset, Next should return the first entry again")
}

func TestRandomBalancer(t *testing.T) {
	entries := []Entry{
		{Index: 1, Key: "key1", Secret: "secret1"},
		{Index: 2, Key: "key2", Secret: "secret2"},
		{Index: 3, Key: "key3", Secret: "secret3"},
	}
	source := &Source{Entries: entries}

	balancer := NewRandomBalancer(source)

	hitCounts := make(map[int]int)
	for i := 0; i < 1000; i++ {
		entry := balancer.Next()
		assert.NotNil(t, entry, "Next should not return nil")
		hitCounts[entry.Index]++
	}

	assert.Equal(t, len(entries), len(hitCounts), "All entries should be selected at least once")

	for _, count := range hitCounts {
		assert.Greater(t, count, 0, "Each entry should have been hit at least once")
	}

	stats := balancer.GetStats()
	totalRate := 0.0
	for _, rate := range stats {
		totalRate += rate
	}
	assert.InDelta(t, 1.0, totalRate, 0.01, "Total hit rate should be approximately 1.0")
}
