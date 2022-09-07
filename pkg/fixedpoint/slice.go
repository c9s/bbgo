package fixedpoint

type Slice []Value

func (s Slice) Reduce(init Value, reducer Reducer) Value {
	return Reduce(s, init, reducer)
}

// Defaults to ascending sort
func (s Slice) Len() int           { return len(s) }
func (s Slice) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s Slice) Less(i, j int) bool { return s[i].Compare(s[j]) < 0 }

type Ascending []Value

func (s Ascending) Len() int           { return len(s) }
func (s Ascending) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s Ascending) Less(i, j int) bool { return s[i].Compare(s[j]) < 0 }

type Descending []Value

func (s Descending) Len() int           { return len(s) }
func (s Descending) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s Descending) Less(i, j int) bool { return s[i].Compare(s[j]) > 0 }
