package types

type FilterResult struct {
	a      Series
	b      func(int, float64) bool
	length int
	c      []int
}

func (f *FilterResult) Last(j int) float64 {
	if j >= f.length {
		return 0
	}
	if len(f.c) > j {
		return f.a.Last(f.c[j])
	}
	l := f.a.Length()
	k := len(f.c)
	i := 0
	if k > 0 {
		i = f.c[k-1] + 1
	}
	for ; i < l; i++ {
		tmp := f.a.Last(i)
		if f.b(i, tmp) {
			f.c = append(f.c, i)
			if j == k {
				return tmp
			}
			k++
		}
	}
	return 0
}

func (f *FilterResult) Index(j int) float64 {
	return f.Last(j)
}

func (f *FilterResult) Length() int {
	return f.length
}

// Filter function filters Series by using a boolean function.
// When the boolean function returns true, the Series value at index i will be included in the returned Series.
// The returned Series will find at most `length` latest matching elements from the input Series.
// Query index larger or equal than length from the returned Series will return 0 instead.
// Notice that any Update on the input Series will make the previously returned Series outdated.
func Filter(a Series, b func(i int, value float64) bool, length int) SeriesExtend {
	return NewSeries(&FilterResult{a, b, length, nil})
}
