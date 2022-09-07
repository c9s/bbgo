package fixedpoint

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSortInterface(t *testing.T) {
	slice := Slice{
		NewFromInt(7),
		NewFromInt(3),
		NewFromInt(1),
		NewFromInt(2),
		NewFromInt(5),
	}
	sort.Sort(slice)
	assert.Equal(t, "1", slice[0].String())
	assert.Equal(t, "2", slice[1].String())
	assert.Equal(t, "3", slice[2].String())
	assert.Equal(t, "5", slice[3].String())

	sort.Sort(Descending(slice))
	assert.Equal(t, "7", slice[0].String())
	assert.Equal(t, "5", slice[1].String())
	assert.Equal(t, "3", slice[2].String())
	assert.Equal(t, "2", slice[3].String())

	sort.Sort(Ascending(slice))
	assert.Equal(t, "1", slice[0].String())
	assert.Equal(t, "2", slice[1].String())
	assert.Equal(t, "3", slice[2].String())
	assert.Equal(t, "5", slice[3].String())
}
