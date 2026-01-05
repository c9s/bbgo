package skiplist

import (
	"math/rand"
	"time"
)

// Comparator compares a and b and returns:
//
//	-1 if a < b, 0 if a == b, +1 if a > b
type Comparator[K any] func(a, b K) int

// IntAscending is a default comparator for int Keys in ascending order.
// Returns -1 if a < b, 0 if a == b, +1 if a > b.
// Usage:
//
//	sl := skiplist.New[int, V](skiplist.IntAscending)
func IntAscending(a, b int) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

// IntDescending is a default comparator for int Keys in descending order.
// Returns -1 if a > b, 0 if a == b, +1 if a < b.
// Usage:
//
//	sl := skiplist.New[int, V](skiplist.IntDescending)
func IntDescending(a, b int) int {
	if a > b {
		return -1
	}
	if a < b {
		return 1
	}
	return 0
}

// SkipList is a generic skip list implementation that supports custom Key/Value types
// and customizable per-level segment lengths that control Node height probabilities.
//
// Segment lengths define the probability to advance to the next level when generating
// a Node height. At level i (0-based), the promotion probability is approximately 1/segmentLengths[i].
// If no segment lengths are provided, a default geometric distribution with factor 2 is used.
type SkipList[K any, V any] struct {
	cmp      Comparator[K]
	head     *Node[K, V]
	level    int // current highest non-empty level (1..maxLevel)
	maxLevel int
	length   int
	rnd      *rand.Rand
	segLens  []int
}

type Node[K any, V any] struct {
	Key   K
	Value V
	next  []*Node[K, V] // size == height of this Node (unexported to hide internals)
}

// New creates a new SkipList with the provided comparator and optional per-level segment lengths.
// The number of segment lengths controls the maximum level; each Value should be >= 2.
// If segmentLengths is empty, a default of 32 levels with segment length 2 is used.
func New[K any, V any](cmp Comparator[K], segmentLengths ...int) *SkipList[K, V] {
	if cmp == nil {
		panic("skiplist: comparator must not be nil")
	}

	if len(segmentLengths) == 0 {
		// Default: 32 levels, promotion probability ~1/2 at each level.
		segmentLengths = make([]int, 32)
		for i := range segmentLengths {
			segmentLengths[i] = 2
		}
	}

	// sanitize segment lengths (must be >=2)
	for i, v := range segmentLengths {
		if v < 2 {
			segmentLengths[i] = 2
		}
	}

	sl := &SkipList[K, V]{
		cmp:      cmp,
		head:     &Node[K, V]{next: make([]*Node[K, V], len(segmentLengths))},
		level:    1,
		maxLevel: len(segmentLengths),
		rnd:      rand.New(rand.NewSource(time.Now().UnixNano())),
		segLens:  segmentLengths,
	}
	return sl
}

// Seed allows setting the random seed used for level generation.
func (sl *SkipList[K, V]) Seed(seed int64) {
	sl.rnd = rand.New(rand.NewSource(seed))
}

// Len returns the number of elements in the skip list.
func (sl *SkipList[K, V]) Len() int { return sl.length }

// Get finds and returns the Value associated with Key.
func (sl *SkipList[K, V]) Get(key K) (V, bool) {
	x := sl.head
	// search from top level down
	for i := sl.level - 1; i >= 0; i-- {
		for nx := x.next[i]; nx != nil && sl.cmp(nx.Key, key) < 0; nx = x.next[i] {
			x = nx
		}
	}
	// Now x is the greatest Node < Key at level 0
	x = x.next[0]
	if x != nil && sl.cmp(x.Key, key) == 0 {
		return x.Value, true
	}
	var zero V
	return zero, false
}

// Set inserts or updates the Key with the given Value.
// It returns (oldValue, true) if the Key existed and was replaced; otherwise (zero, false).
func (sl *SkipList[K, V]) Set(key K, value V) (V, bool) {
	update := make([]*Node[K, V], sl.maxLevel)
	x := sl.head
	for i := sl.level - 1; i >= 0; i-- {
		for nx := x.next[i]; nx != nil && sl.cmp(nx.Key, key) < 0; nx = x.next[i] {
			x = nx
		}
		update[i] = x
	}

	// Check if Key exists at level 0
	if nx := x.next[0]; nx != nil && sl.cmp(nx.Key, key) == 0 {
		old := nx.Value
		nx.Value = value
		return old, true
	}

	// Insert new Node
	lvl := sl.randomLevel()
	if lvl > sl.level {
		// initialize update references for the new levels with head
		for i := sl.level; i < lvl; i++ {
			update[i] = sl.head
		}
		sl.level = lvl
	}
	n := &Node[K, V]{Key: key, Value: value, next: make([]*Node[K, V], lvl)}
	for i := 0; i < lvl; i++ {
		n.next[i] = update[i].next[i]
		update[i].next[i] = n
	}
	sl.length++
	var zero V
	return zero, false
}

// Delete removes the Key from the skip list, returning its Value and true if present.
func (sl *SkipList[K, V]) Delete(key K) (V, bool) {
	update := make([]*Node[K, V], sl.maxLevel)
	x := sl.head
	for i := sl.level - 1; i >= 0; i-- {
		for nx := x.next[i]; nx != nil && sl.cmp(nx.Key, key) < 0; nx = x.next[i] {
			x = nx
		}
		update[i] = x
	}

	target := x.next[0]
	if target == nil || sl.cmp(target.Key, key) != 0 {
		var zero V
		return zero, false
	}

	// unlink at each level
	for i := 0; i < len(target.next); i++ {
		if update[i].next[i] == target {
			update[i].next[i] = target.next[i]
		}
	}

	// adjust current level if needed
	for sl.level > 1 && sl.head.next[sl.level-1] == nil {
		sl.level--
	}
	sl.length--
	return target.Value, true
}

// Ascend iterates over all Key-Value pairs in ascending Key order.
// The iteration stops if fn returns false.
func (sl *SkipList[K, V]) Ascend(fn func(key K, value V) bool) {
	for n := sl.head.next[0]; n != nil; n = n.next[0] {
		if !fn(n.Key, n.Value) {
			return
		}
	}
}

// AscendFrom iterates from the first element with Key >= start.
func (sl *SkipList[K, V]) AscendFrom(start K, fn func(key K, value V) bool) {
	// Find first >= start
	x := sl.head
	for i := sl.level - 1; i >= 0; i-- {
		for nx := x.next[i]; nx != nil && sl.cmp(nx.Key, start) < 0; nx = x.next[i] {
			x = nx
		}
	}
	for n := x.next[0]; n != nil; n = n.next[0] {
		if !fn(n.Key, n.Value) {
			return
		}
	}
}

// Head returns up to the first n elements in ascending Key order as Node values.
// The returned Node values are copies that expose only Key and Value; the internal
// linkage (next) is not exported and is left nil in returned values.
// If n <= 0 or the list is empty, it returns an empty slice.
// If n >= Len(), it returns all elements.
func (sl *SkipList[K, V]) Head(n int) []Node[K, V] {
	if n <= 0 || sl.length == 0 {
		return nil
	}
	if n > sl.length {
		n = sl.length
	}
	res := make([]Node[K, V], 0, n)
	for nd := sl.head.next[0]; nd != nil && len(res) < n; nd = nd.next[0] {
		res = append(res, Node[K, V]{Key: nd.Key, Value: nd.Value})
	}
	return res
}

// Min returns the smallest Key/Value pair.
func (sl *SkipList[K, V]) Min() (K, V, bool) {
	n := sl.head.next[0]
	if n == nil {
		var zk K
		var zv V
		return zk, zv, false
	}
	return n.Key, n.Value, true
}

// Max returns the largest Key/Value pair.
func (sl *SkipList[K, V]) Max() (K, V, bool) {
	x := sl.head
	for i := sl.level - 1; i >= 0; i-- {
		for nx := x.next[i]; nx != nil; nx = x.next[i] {
			x = nx
		}
	}
	if x == sl.head {
		var zk K
		var zv V
		return zk, zv, false
	}
	return x.Key, x.Value, true
}

// randomLevel generates a level in [1, maxLevel], using segLens probabilities.
func (sl *SkipList[K, V]) randomLevel() int {
	lvl := 1
	// try to promote to higher levels
	// For level i (0-based), promote to i+2 with probability ~1/segLens[i]
	for lvl < sl.maxLevel {
		seg := sl.segLens[lvl-1]
		if seg < 2 {
			seg = 2
		}
		if sl.rnd.Intn(seg) == 0 {
			lvl++
		} else {
			break
		}
	}
	return lvl
}
