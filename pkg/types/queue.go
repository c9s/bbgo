package types

// Super basic Series type that simply holds the float64 data
// with size limit (the only difference compare to float64slice)
type Queue struct {
	SeriesBase
	arr  []float64
	size int
}

func NewQueue(size int) *Queue {
	out := &Queue{
		arr:  make([]float64, 0, size),
		size: size,
	}
	out.SeriesBase.Series = out
	return out
}

func (inc *Queue) Last(i int) float64 {
	if i < 0 || len(inc.arr)-i-1 < 0 {
		return 0
	}

	return inc.arr[len(inc.arr)-1-i]
}

func (inc *Queue) Index(i int) float64 {
	return inc.Last(i)
}

func (inc *Queue) Length() int {
	return len(inc.arr)
}

func (inc *Queue) Clone() *Queue {
	out := &Queue{
		arr:  inc.arr[:],
		size: inc.size,
	}
	out.SeriesBase.Series = out
	return out
}

func (inc *Queue) Update(v float64) {
	inc.arr = append(inc.arr, v)
	if len(inc.arr) > inc.size {
		inc.arr = inc.arr[len(inc.arr)-inc.size:]
	}
}

var _ UpdatableSeriesExtend = &Queue{}
