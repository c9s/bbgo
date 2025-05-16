package apikey

import (
	"sync"
)

type RoundTripBalancer struct {
	source *Source
	mu     sync.Mutex
	pos    int
}

func NewRoundTripBalancer(source *Source) *RoundTripBalancer {
	return &RoundTripBalancer{source: source}
}

func (b *RoundTripBalancer) Next() *Entry {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(b.source.Entries) == 0 {
		return nil
	}

	entry := &b.source.Entries[b.pos]
	b.pos = (b.pos + 1) % len(b.source.Entries)
	return entry
}

func (b *RoundTripBalancer) Reset() {
	b.mu.Lock()
	b.pos = 0
	b.mu.Unlock()
}

func (b *RoundTripBalancer) Peek() *Entry {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(b.source.Entries) == 0 {
		return nil
	}

	return &b.source.Entries[b.pos]
}
