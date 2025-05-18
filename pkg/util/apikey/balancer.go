package apikey

import (
	"math/rand/v2"
	"strconv"
	"sync"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
)

var hitCounterMetric = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "roundtrip_balancer_hit_count",
		Help: "Hit count for each API key entry",
	},
	[]string{"uuid", "index"},
)

var hitRateMetric = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "roundtrip_balancer_hit_rate",
		Help: "Hit rate for each API key entry",
	},
	[]string{"uuid", "index"},
)

func init() {
	prometheus.MustRegister(hitCounterMetric, hitRateMetric)
}

type RoundTripBalancer struct {
	source     *Source
	mu         sync.Mutex
	pos        int
	hitCounter map[int]int // Tracks hit counts for each entry
	uuid       string
}

func NewRoundTripBalancer(source *Source) *RoundTripBalancer {

	return &RoundTripBalancer{
		source:     source,
		hitCounter: make(map[int]int, source.Len()),
		uuid:       uuid.NewString(),
	}
}

func (b *RoundTripBalancer) Next() *Entry {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(b.source.Entries) == 0 {
		return nil
	}

	entry := &b.source.Entries[b.pos]
	b.hitCounter[entry.Index]++ // Increment hit counter for the current entry
	b.pos = (b.pos + 1) % len(b.source.Entries)

	hitCounterMetric.WithLabelValues(b.uuid, strconv.Itoa(entry.Index)).Inc()
	b.updateMetrics() // Update hitRateMetric
	return entry
}

func (b *RoundTripBalancer) Reset() {
	b.mu.Lock()
	b.pos = 0
	b.hitCounter = make(map[int]int, b.source.Len())
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

func (b *RoundTripBalancer) updateMetrics() {
	totalHits := 0
	for _, count := range b.hitCounter {
		totalHits += count
	}

	for _, entry := range b.source.Entries {
		hitCount := b.hitCounter[entry.Index]
		if totalHits > 0 {
			hitRate := float64(hitCount) / float64(totalHits)
			hitRateMetric.WithLabelValues(b.uuid, strconv.Itoa(entry.Index)).Set(hitRate)
		} else {
			hitRateMetric.WithLabelValues(b.uuid, strconv.Itoa(entry.Index)).Set(0)
		}
	}
}

func (b *RoundTripBalancer) GetStats() map[int]float64 {
	b.mu.Lock()
	defer b.mu.Unlock()

	totalHits := 0
	for _, count := range b.hitCounter {
		totalHits += count
	}

	stats := make(map[int]float64)
	for _, entry := range b.source.Entries {
		hitCount := b.hitCounter[entry.Index]
		if totalHits > 0 {
			hitRate := float64(hitCount) / float64(totalHits)
			stats[entry.Index] = hitRate
		} else {
			stats[entry.Index] = 0
		}
	}

	return stats
}

type RandomBalancer struct {
	source     *Source
	mu         sync.Mutex
	hitCounter map[int]int
	uuid       string
}

func NewRandomBalancer(source *Source) *RandomBalancer {
	return &RandomBalancer{
		source:     source,
		hitCounter: make(map[int]int, source.Len()),
		uuid:       uuid.NewString(),
	}
}

func (b *RandomBalancer) Next() *Entry {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(b.source.Entries) == 0 {
		return nil
	}

	// Randomly select an entry
	index := rand.IntN(len(b.source.Entries))
	entry := &b.source.Entries[index]

	// Update hit counter and metrics
	b.hitCounter[entry.Index]++
	hitCounterMetric.WithLabelValues(b.uuid, strconv.Itoa(entry.Index)).Inc()
	b.updateMetrics()

	return entry
}

func (b *RandomBalancer) GetStats() map[int]float64 {
	b.mu.Lock()
	defer b.mu.Unlock()

	totalHits := 0
	for _, count := range b.hitCounter {
		totalHits += count
	}

	stats := make(map[int]float64)
	for _, entry := range b.source.Entries {
		hitCount := b.hitCounter[entry.Index]
		if totalHits > 0 {
			hitRate := float64(hitCount) / float64(totalHits)
			stats[entry.Index] = hitRate
		} else {
			stats[entry.Index] = 0
		}
	}

	return stats
}

func (b *RandomBalancer) updateMetrics() {
	totalHits := 0
	for _, count := range b.hitCounter {
		totalHits += count
	}

	for _, entry := range b.source.Entries {
		hitCount := b.hitCounter[entry.Index]
		if totalHits > 0 {
			hitRate := float64(hitCount) / float64(totalHits)
			hitRateMetric.WithLabelValues(b.uuid, strconv.Itoa(entry.Index)).Set(hitRate)
		} else {
			hitRateMetric.WithLabelValues(b.uuid, strconv.Itoa(entry.Index)).Set(0)
		}
	}
}
