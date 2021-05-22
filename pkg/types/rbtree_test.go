package types

import (
	"testing"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/stretchr/testify/assert"
)

func TestTree_Copy(t *testing.T) {
	tree := NewRBTree()
	tree.Insert(fixedpoint.NewFromFloat(3000.0), fixedpoint.NewFromFloat(1.0))
	assert.NotNil(t, tree.Root)

	tree.Insert(fixedpoint.NewFromFloat(4000.0), fixedpoint.NewFromFloat(2.0))
	tree.Insert(fixedpoint.NewFromFloat(2000.0), fixedpoint.NewFromFloat(3.0))

	newTree := tree.Copy()
	node1 := newTree.Search(fixedpoint.NewFromFloat(2000.0))
	assert.NotNil(t, node1)
	assert.Equal(t, fixedpoint.NewFromFloat(2000.0), node1.Key)
	assert.Equal(t, fixedpoint.NewFromFloat(3.0), node1.Value)

	node2 := newTree.Search(fixedpoint.NewFromFloat(3000.0))
	assert.NotNil(t, node2)
	assert.Equal(t, fixedpoint.NewFromFloat(3000.0), node2.Key)
	assert.Equal(t, fixedpoint.NewFromFloat(1.0), node2.Value)

	node3 := newTree.Search(fixedpoint.NewFromFloat(4000.0))
	assert.NotNil(t, node3)
	assert.Equal(t, fixedpoint.NewFromFloat(4000.0), node3.Key)
	assert.Equal(t, fixedpoint.NewFromFloat(2.0), node3.Value)
}

func TestTree(t *testing.T) {
	tree := NewRBTree()
	tree.Insert(fixedpoint.NewFromFloat(3000.0), fixedpoint.NewFromFloat(10.0))
	assert.NotNil(t, tree.Root)

	tree.Insert(fixedpoint.NewFromFloat(4000.0), fixedpoint.NewFromFloat(10.0))
	tree.Insert(fixedpoint.NewFromFloat(2000.0), fixedpoint.NewFromFloat(10.0))

	// root is always black
	assert.Equal(t, fixedpoint.NewFromFloat(3000.0), tree.Root.Key)
	assert.Equal(t, Black, tree.Root.Color)

	assert.Equal(t, fixedpoint.NewFromFloat(2000.0), tree.Root.Left.Key)
	assert.Equal(t, Red, tree.Root.Left.Color)

	assert.Equal(t, fixedpoint.NewFromFloat(4000.0), tree.Root.Right.Key)
	assert.Equal(t, Red, tree.Root.Right.Color)

	// should rotate
	tree.Insert(fixedpoint.NewFromFloat(1500.0), fixedpoint.NewFromFloat(10.0))
	tree.Insert(fixedpoint.NewFromFloat(1000.0), fixedpoint.NewFromFloat(10.0))

	deleted := tree.Delete(fixedpoint.NewFromFloat(1000.0))
	assert.True(t, deleted)

	deleted = tree.Delete(fixedpoint.NewFromFloat(1500.0))
	assert.True(t, deleted)

}
