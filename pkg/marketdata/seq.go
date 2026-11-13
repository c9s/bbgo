package marketdata

import "iter"

// Seq adapts a Cursor to a range-over-func loop:
//
//	for ev := range marketdata.Seq(cur) {
//	    ...
//	}
//	if err := cur.Err(); err != nil { ... }
//
// The iteration stops at exhaustion or on the first error; call Err afterwards
// to tell the two apart. This is a convenience for consumers; Merge uses the
// Cursor interface directly, because a k-way merge needs to peek the head of
// every input and advance exactly one of them.
func Seq(c Cursor) iter.Seq[*Event] {
	return func(yield func(*Event) bool) {
		for c.Next() {
			if !yield(c.Event()) {
				return
			}
		}
	}
}
