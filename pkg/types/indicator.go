package types

// Float64Indicator is the indicators (SMA and EWMA) that we want to use are returning float64 data.
type Float64Indicator interface {
	Last() float64
}

// The interface maps to pinescript basic type `series`
// Access the internal historical data from the latest to the oldest
// Index(0) always maps to Last()
type Series interface {
	Last() float64
	Index(int) float64
	Length() int
}

// The interface maps to pinescript basic type `series` for bool type
// Access the internal historical data from the latest to the oldest
// Index(0) always maps to Last()
type BoolSeries interface {
	Last() bool
	Index(int) bool
	Length() int
}

// The result structure that maps to the crossing result of `CrossOver` and `CrossUnder`
// Accessible through BoolSeries interface
type CrossResult struct {
	a      Series
	b      Series
	isOver bool
}

func (c *CrossResult) Last() bool {
	if c.Length() == 0 {
		return false
	}
	if c.isOver {
		return c.a.Last()-c.b.Last() > 0 && c.a.Index(1)-c.b.Index(1) < 0
	} else {
		return c.a.Last()-c.b.Last() < 0 && c.a.Index(1)-c.b.Index(1) > 0
	}
}

func (c *CrossResult) Index(i int) bool {
	if i >= c.Length() {
		return false
	}
	if c.isOver {
		return c.a.Index(i)-c.b.Index(i) > 0 && c.a.Index(i-1)-c.b.Index(i-1) < 0
	} else {
		return c.a.Index(i)-c.b.Index(i) < 0 && c.a.Index(i-1)-c.b.Index(i-1) > 0
	}
}

func (c *CrossResult) Length() int {
	la := c.a.Length()
	lb := c.b.Length()
	if la > lb {
		return lb
	}
	return la
}

// a series cross above b series.
// If in current KLine, a is higher than b, and in previous KLine, a is lower than b, then return true.
// Otherwise return false.
// If accessing index <= length, will always return false
func CrossOver(a Series, b Series) BoolSeries {
	return &CrossResult{a, b, true}
}

// a series cross under b series.
// If in current KLine, a is lower than b, and in previous KLine, a is higher than b, then return true.
// Otherwise return false.
// If accessing index <= length, will always return false
func CrossUnder(a Series, b Series) BoolSeries {
	return &CrossResult{a, b, false}
}

func Highest(a Series, lookback int) float64 {
	if lookback > a.Length() {
		lookback = a.Length()
	}
	highest := a.Last()
	for i := 1; i < lookback; i++ {
		current := a.Index(i)
		if highest < current {
			highest = current
		}
	}
	return highest
}

func Lowest(a Series, lookback int) float64 {
	if lookback > a.Length() {
		lookback = a.Length()
	}
	lowest := a.Last()
	for i := 1; i < lookback; i++ {
		current := a.Index(i)
		if lowest > current {
			lowest = current
		}
	}
	return lowest
}
