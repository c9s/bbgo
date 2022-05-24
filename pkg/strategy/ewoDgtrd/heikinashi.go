package ewoDgtrd

import (
	"fmt"
	"math"

	"github.com/c9s/bbgo/pkg/types"
)

type Queue struct {
	arr  []float64
	size int
}

func NewQueue(size int) *Queue {
	return &Queue{
		arr:  make([]float64, 0, size),
		size: size,
	}
}

func (inc *Queue) Last() float64 {
	if len(inc.arr) == 0 {
		return 0
	}
	return inc.arr[len(inc.arr)-1]
}

func (inc *Queue) Index(i int) float64 {
	if len(inc.arr)-i-1 < 0 {
		return 0
	}
	return inc.arr[len(inc.arr)-i-1]
}

func (inc *Queue) Length() int {
	return len(inc.arr)
}

func (inc *Queue) Update(v float64) {
	inc.arr = append(inc.arr, v)
	if len(inc.arr) > inc.size {
		inc.arr = inc.arr[len(inc.arr)-inc.size:]
	}
}

type HeikinAshi struct {
	Close  *Queue
	Open   *Queue
	High   *Queue
	Low    *Queue
	Volume *Queue
}

func NewHeikinAshi(size int) *HeikinAshi {
	return &HeikinAshi{
		Close:  NewQueue(size),
		Open:   NewQueue(size),
		High:   NewQueue(size),
		Low:    NewQueue(size),
		Volume: NewQueue(size),
	}
}

func (s *HeikinAshi) Print() string {
	return fmt.Sprintf("Heikin c: %.3f, o: %.3f, h: %.3f, l: %.3f, v: %.3f",
		s.Close.Last(),
		s.Open.Last(),
		s.High.Last(),
		s.Low.Last(),
		s.Volume.Last())
}

func (inc *HeikinAshi) Update(kline types.KLine) {
	open := kline.Open.Float64()
	cloze := kline.Close.Float64()
	high := kline.High.Float64()
	low := kline.Low.Float64()
	newClose := (open + high + low + cloze) / 4.
	newOpen := (inc.Open.Last() + inc.Close.Last()) / 2.
	inc.Close.Update(newClose)
	inc.Open.Update(newOpen)
	inc.High.Update(math.Max(math.Max(high, newOpen), newClose))
	inc.Low.Update(math.Min(math.Min(low, newOpen), newClose))
	inc.Volume.Update(kline.Volume.Float64())
}
