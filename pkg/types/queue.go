package types

// Super basic Series type that simply holds the float64 data
// with size limit (the only difference compare to float64slice)
type Queue struct {
	SeriesBase
	arr      []float64
	size     int // size of the queue
	start    int // start index of the queue
	last     int // last index of the queue
	realSize int // real size of the queue
}

// the real size of the queue is the next power of 2
func getRealSize(n int) int {
	if n <= 1 {
		return 1
	}
	n--
	n |= n >> 1
	n |= n >> 2
	n |= n >> 4
	n |= n >> 8
	n |= n >> 16
	if ^uint(0)>>32 != 0 { // 64 bit
		n |= n >> 32
	}
	return n
}

func NewQueue(size int) *Queue {
	realSize := getRealSize(size)
	out := &Queue{
		arr:      make([]float64, 0, realSize+1),
		size:     size,
		start:    0,
		last:     -1,
		realSize: realSize,
	}
	out.SeriesBase.Series = out
	return out
}

func (inc *Queue) Last(i int) float64 {
	if i < 0 || len(inc.arr)-i-1 < 0 {
		return 0
	}

	return inc.arr[(inc.last-i)&inc.realSize]
}

func (inc *Queue) Index(i int) float64 {
	return inc.Last(i)
}

func (inc *Queue) Length() int {
	if inc.size < len(inc.arr) {
		return inc.size
	}
	return len(inc.arr)
}

func (inc *Queue) Clone() *Queue {
	arrCopy := make([]float64, len(inc.arr), cap(inc.arr))
	copy(arrCopy, inc.arr)
	out := &Queue{
		arr:      arrCopy,
		size:     inc.size,
		start:    inc.start,
		last:     inc.last,
		realSize: inc.realSize,
	}
	out.SeriesBase.Series = out
	return out
}

func (inc *Queue) Update(v float64) {
	c := len(inc.arr)
	if c <= inc.realSize {
		inc.arr = append(inc.arr, v)
		inc.last++
	} else {
		if inc.size == 0 {
			return
		}
		inc.arr[inc.start] = v
		inc.start = (inc.start + 1) & inc.realSize
		inc.last = (inc.last + 1) & inc.realSize
	}
}

var _ UpdatableSeriesExtend = &Queue{}
