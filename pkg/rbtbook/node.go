package rbtbook

import "github.com/c9s/bbgo/pkg/fixedpoint"

type Color bool

const (
	Red   = Color(false)
	Black = Color(true)
)

type Tree struct {
	Root *Node
}

func (t *Tree) UpdateOrInsert(key, val fixedpoint.Value) {
	var y *Node = nil
	var x = t.Root
	var node = &Node{
		Price:  key,
		Volume: val,
	}

	for x != nil {
		y = x

		// matched an existing node
		if node.Price == x.Price {
			x.Volume = val
			return
		} else if node.Price < x.Price {
			x = x.Left
		} else {
			x = x.Right
		}
	}

	node.Parent = y

	if y == nil {
		t.Root = node
	} else if node.Price < y.Price {
		y.Left = node
	} else {
		y.Right = node
	}

	node.Left = nil
	node.Right = nil
	node.Color = Red
}

func InsertFixup(tree *Tree, node *Node) {

}

/*
Node
A red node always has black children.
A black node may have red or black children
*/
type Node struct {
	Left, Right, Parent *Node
	Color               Color
	Price               fixedpoint.Value
	Volume              fixedpoint.Value
}

// RotateLeft
// x is the axes of rotation, y is the node that will be replace x's position.
// we need to:
// 1. move y's left child to the x's right child
// 2. change y's parent to x's parent
// 3. change x's parent to y
func RotateLeft(tree *Tree, x *Node) {
	var y = x.Right
	x.Right = y.Left

	if y.Left != nil {
		y.Left.Parent = x
	}

	y.Parent = x.Parent

	if x.Parent == nil {
		tree.Root = y
	} else if x == x.Parent.Left {
		x.Parent.Left = y
	} else {
		x.Parent.Right = y
	}

	y.Left = x
	x.Parent = y
}

func RotateRight(tree *Tree, y *Node) {
	x := y.Left
	y.Left = x.Right

	if x.Right != nil {
		x.Right.Parent = y
	}

	x.Parent = y.Parent

	if y.Parent == nil {
		tree.Root = x
	} else if y == y.Parent.Left {
		y.Parent.Left = x
	} else {
		y.Parent.Right = x
	}

	x.Right = y
	y.Parent = x
}
