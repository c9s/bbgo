package types

import (
	"testing"
)

type OldQueue struct {
	SeriesBase
	arr  []float64
	size int
}

func (inc *OldQueue) Update(value float64) {
	inc.arr = append(inc.arr, value)
	if len(inc.arr) > inc.size {
		inc.arr = inc.arr[len(inc.arr)-inc.size:]
	}
}

func (inc *OldQueue) Last(i int) float64 {
	if i < 0 || i >= len(inc.arr) {
		return 0
	}
	return inc.arr[len(inc.arr)-1-i]
}

func BenchmarkQueue(b *testing.B) {
	q := NewQueue(1000)
	b.ResetTimer()
	b.Run("bbgo-latest-queue", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			q.Update(float64(i))
		}
	})

	o := &OldQueue{
		arr:  make([]float64, 0, 1000),
		size: 1000,
	}
	b.ResetTimer()
	b.Run("bbgo-last-queue", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			o.Update(float64(i))
		}
	})

}

func BenchmarkLargeQueue(b *testing.B) {
	q := NewQueue(200000)
	b.ResetTimer()
	b.Run("bbgo-latest-queue", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			q.Update(float64(i))
		}
	})

	o := &OldQueue{
		arr:  make([]float64, 0, 200000),
		size: 200000,
	}
	b.ResetTimer()
	b.Run("bbgo-last-queue", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			o.Update(float64(i))
		}
	})
}

func BenchmarkQueueLast(b *testing.B) {
	q := NewQueue(1000)
	b.Run("bbgo-latest-queue", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			q.Update(float64(i))
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			q.Last(i)
		}
	})

	o := &OldQueue{
		arr:  make([]float64, 0, 1000),
		size: 1000,
	}
	b.Run("bbgo-last-queue", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			o.Update(float64(i))
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			o.Last(i)
		}
	})
}
