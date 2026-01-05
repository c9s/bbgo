package skiplist

import (
	"fmt"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
)

func intCmp(a, b int) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

func TestSkipList_SetGetUpdateDelete(t *testing.T) {
	sl := New[int, string](intCmp, 2, 3, 5, 7)
	sl.Seed(42) // stabilize randomization (not strictly required for correctness)

	// Insert
	pairs := map[int]string{
		5: "five",
		1: "one",
		3: "three",
		2: "two",
		4: "four",
	}
	for k, v := range pairs {
		_, replaced := sl.Set(k, v)
		assert.False(t, replaced)
	}
	assert.Equal(t, 5, sl.Len())

	// Get
	for k, v := range pairs {
		got, ok := sl.Get(k)
		assert.True(t, ok)
		assert.Equal(t, v, got)
	}

	// Update
	old, replaced := sl.Set(3, "THREE")
	assert.True(t, replaced)
	assert.Equal(t, "three", old)
	got, ok := sl.Get(3)
	assert.True(t, ok)
	assert.Equal(t, "THREE", got)
	assert.Equal(t, 5, sl.Len())

	// Min/Max
	mink, minv, ok := sl.Min()
	assert.True(t, ok)
	assert.Equal(t, 1, mink)
	assert.Equal(t, "one", minv)
	maxk, maxv, ok := sl.Max()
	assert.True(t, ok)
	assert.Equal(t, 5, maxk)
	assert.Equal(t, "five", maxv)

	// Ascend order
	keys := make([]int, 0, sl.Len())
	values := make([]string, 0, sl.Len())
	sl.Ascend(func(k int, v string) bool {
		keys = append(keys, k)
		values = append(values, v)
		return true
	})
	expectedKeys := []int{1, 2, 3, 4, 5}
	assert.Equal(t, expectedKeys, keys)

	// AscendFrom
	fromKeys := make([]int, 0)
	sl.AscendFrom(3, func(k int, v string) bool {
		fromKeys = append(fromKeys, k)
		return true
	})
	assert.Equal(t, []int{3, 4, 5}, fromKeys)

	// Delete
	val, ok := sl.Delete(5)
	assert.True(t, ok)
	assert.Equal(t, "five", val)
	assert.Equal(t, 4, sl.Len())
	_, ok = sl.Get(5)
	assert.False(t, ok)
}

func TestSkipList_ManyElements_OrderAndFind(t *testing.T) {
	sl := New[int, int](intCmp) // default segment lengths
	sl.Seed(123)

	// insert 100 numbers in descending order
	for i := 200; i >= 101; i-- {
		sl.Set(i, i*i)
	}
	assert.Equal(t, 100, sl.Len())

	// verify get
	for i := 101; i <= 200; i++ {
		v, ok := sl.Get(i)
		assert.True(t, ok)
		assert.Equal(t, i*i, v)
	}

	// verify iteration order
	keys := make([]int, 0, 100)
	sl.Ascend(func(k, v int) bool {
		keys = append(keys, k)
		return true
	})
	assert.Equal(t, 100, len(keys))
	sorted := append([]int(nil), keys...)
	sort.Ints(sorted)
	assert.Equal(t, sorted, keys)

	// delete some
	for i := 120; i <= 130; i++ {
		_, ok := sl.Delete(i)
		assert.True(t, ok)
	}
	assert.Equal(t, 89, sl.Len())
}

func TestSkipList_HeadTail(t *testing.T) {
	sl := New[int, string](intCmp)

	// empty cases
	assert.Equal(t, 0, len(sl.Head(3)))
	assert.Equal(t, 0, len(sl.Head(0)))
	assert.Equal(t, 0, len(sl.Head(-1)))

	// fill with 1..5
	for i := 1; i <= 5; i++ {
		sl.Set(i, string(rune('a'+i-1))) // 1:'a', 2:'b', ...
	}

	// Head beyond length => all
	headAll := sl.Head(10)
	assert.Equal(t, 5, len(headAll))
	for i, p := range headAll {
		assert.Equal(t, i+1, p.Key)
		assert.Equal(t, string(rune('a'+i)), p.Value)
	}

	// Head(3)
	head3 := sl.Head(3)
	assert.Equal(t, 3, len(head3))
	assert.Equal(t, 1, head3[0].Key)
	assert.Equal(t, 2, head3[1].Key)
	assert.Equal(t, 3, head3[2].Key)
}

// ExampleIntAscending demonstrates creating a SkipList with int keys
// in ascending order using the provided IntAscending comparator.
func ExampleIntAscending() {
	sl := New[int, string](IntAscending)

	// Insert keys out of order
	sl.Set(3, "c")
	sl.Set(1, "a")
	sl.Set(2, "b")

	// Iterate in ascending numeric order of keys: 1, 2, 3
	sl.Ascend(func(k int, v string) bool {
		fmt.Printf("%d:%s\n", k, v)
		return true
	})

	// Output:
	// 1:a
	// 2:b
	// 3:c
}

// ExampleIntDescending demonstrates creating a SkipList with int keys
// in descending order using the provided IntDescending comparator.
func ExampleIntDescending() {
	sl := New[int, string](IntDescending)

	// Insert keys out of order
	sl.Set(3, "c")
	sl.Set(1, "a")
	sl.Set(2, "b")

	// Iterate in descending numeric order of keys: 3, 2, 1
	sl.Ascend(func(k int, v string) bool { // Ascend follows the list order defined by the comparator
		fmt.Printf("%d:%s\n", k, v)
		return true
	})

	// Output:
	// 3:c
	// 2:b
	// 1:a
}
