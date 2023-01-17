package bools

type BoolSlice []bool

func New(a ...bool) BoolSlice {
	return BoolSlice(a)
}

func (s *BoolSlice) Push(v bool) {
	*s = append(*s, v)
}

func (s *BoolSlice) Update(v bool) {
	*s = append(*s, v)
}

func (s *BoolSlice) Pop(i int64) (v bool) {
	v = (*s)[i]
	*s = append((*s)[:i], (*s)[i+1:]...)
	return v
}

func (s BoolSlice) Tail(size int) BoolSlice {
	length := len(s)
	if length <= size {
		win := make(BoolSlice, length)
		copy(win, s)
		return win
	}

	win := make(BoolSlice, size)
	copy(win, s[length-size:])
	return win
}

func (s *BoolSlice) Length() int {
	return len(*s)
}

func (s *BoolSlice) Index(i int) bool {
	length := len(*s)
	if length-i < 0 || i < 0 {
		return false
	}
	return (*s)[length-i-1]
}

func (s *BoolSlice) Last() bool {
	length := len(*s)
	if length > 0 {
		return (*s)[length-1]
	}
	return false
}
