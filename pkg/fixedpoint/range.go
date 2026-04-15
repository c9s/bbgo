package fixedpoint

type Range struct {
	Min Value
	Max Value
}

func (r Range) Contains(v Value) bool {
	return v.Compare(r.Min) >= 0 && v.Compare(r.Max) <= 0
}
