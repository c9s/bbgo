package types

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
)

type RBTree struct {
	Root, Neel *RBNode
}

func NewRBTree() *RBTree {
	var neel = &RBNode{
		Color: Black,
	}
	var root = neel
	root.Parent = neel

	return &RBTree{
		Root: root,
		Neel: neel,
	}
}

func (tree *RBTree) Delete(key fixedpoint.Value) bool {
	var del = tree.Search(key)
	if del == nil {
		return false
	}

	// y = the node to be deleted
	// x (the child of the deleted node)
	var x, y *RBNode

	if del.Left == tree.Neel || del.Right == tree.Neel {
		y = del
	} else {
		y = tree.Successor(del)
	}

	if y.Left != tree.Neel {
		x = y.Left
	} else {
		x = y.Right
	}

	x.Parent = y.Parent

	if y.Parent == tree.Neel {
		tree.Root = x
	} else if y == y.Parent.Left {
		y.Parent.Left = x
	} else {
		y.Parent.Right = x
	}

	if y != del {
		del.Key = y.Key
	}

	if y.Color == Black {
		tree.DeleteFixup(x)
	}

	return true
}

func (tree *RBTree) DeleteFixup(current *RBNode) {
	for current != tree.Root && current.Color == Black {
		if current == current.Parent.Left {
			sibling := current.Parent.Right
			if sibling.Color == Red {
				sibling.Color = Black
				current.Parent.Color = Red
				tree.RotateLeft(current.Parent)
				sibling = current.Parent.Right
			}

			// if both are black nodes
			if sibling.Left.Color == Black && sibling.Right.Color == Black {
				sibling.Color = Red
				current = current.Parent
			} else {
				// only one of the child is black
				if sibling.Right.Color == Black {
					sibling.Left.Color = Black
					sibling.Color = Red
					tree.RotateRight(sibling)
					sibling = current.Parent.Right
				}

				sibling.Color = current.Parent.Color
				current.Parent.Color = Black
				sibling.Right.Color = Black
				tree.RotateLeft(current.Parent)
				current = tree.Root
			}
		} else { // if current is right child
			sibling := current.Parent.Left
			if sibling.Color == Red {
				sibling.Color = Black
				current.Parent.Color = Red
				tree.RotateRight(current.Parent)
				sibling = current.Parent.Left
			}

			if sibling.Left.Color == Black && sibling.Right.Color == Black {
				sibling.Color = Red
				current = current.Parent
			} else { // if only one of child is Black

				// the left child of sibling is black, and right child is red
				if sibling.Left.Color == Black {
					sibling.Right.Color = Black
					sibling.Color = Red
					tree.RotateLeft(sibling)
					sibling = current.Parent.Left
				}

				sibling.Color = current.Parent.Color
				current.Parent.Color = Black
				sibling.Left.Color = Black
				tree.RotateRight(current.Parent)
				current = tree.Root
			}
		}
	}

	current.Color = Black
}

func (tree *RBTree) Upsert(key, val fixedpoint.Value) {
	var y = tree.Neel
	var x = tree.Root
	var node = &RBNode{
		Key:   key,
		Value: val,
		Color: Red,
	}

	for x != tree.Neel {
		y = x

		if node.Key == x.Key {
			// found node, skip insert and fix
			x.Value = val
			return
		} else if node.Key < x.Key {
			x = x.Left
		} else {
			x = x.Right
		}
	}

	node.Parent = y

	if y == tree.Neel {
		tree.Root = node
	} else if node.Key < y.Key {
		y.Left = node
	} else {
		y.Right = node
	}

	node.Left = tree.Neel
	node.Right = tree.Neel
	node.Color = Red

	tree.InsertFixup(node)
}

func (tree *RBTree) Insert(key, val fixedpoint.Value) {
	var y = tree.Neel
	var x = tree.Root
	var node = &RBNode{
		Key:   key,
		Value: val,
		Color: Red,
	}

	for x != tree.Neel {
		y = x

		if node.Key < x.Key {
			x = x.Left
		} else {
			x = x.Right
		}
	}

	node.Parent = y

	if y == tree.Neel {
		tree.Root = node
	} else if node.Key < y.Key {
		y.Left = node
	} else {
		y.Right = node
	}

	node.Left = tree.Neel
	node.Right = tree.Neel
	node.Color = Red

	tree.InsertFixup(node)
}

func (tree *RBTree) Search(key fixedpoint.Value) *RBNode {
	var current = tree.Root
	for current != nil && key != current.Key {
		if key < current.Key {
			current = current.Left
		} else {
			current = current.Right
		}
	}
	return current
}

func (tree *RBTree) InsertFixup(current *RBNode) {
	// A red node can't have a red parent, we need to fix it up
	for current.Parent.Color == Red {
		grandParent := current.Parent.Parent
		if current.Parent == grandParent.Left {
			uncle := grandParent.Right
			if uncle.Color == Red {
				current.Parent.Color = Black
				uncle.Color = Black
				grandParent.Color = Red
				current = grandParent
			} else { // if uncle is black
				if current == current.Parent.Right {
					current = current.Parent
					tree.RotateLeft(current)
				}

				current.Parent.Color = Black
				current.Parent.Parent.Color = Red
				tree.RotateRight(current.Parent.Parent)
			}
		} else {
			uncle := current.Parent.Parent.Left
			if uncle.Color == Red {
				current.Parent.Color = Black
				uncle.Color = Black
				current.Parent.Parent.Color = Red
			} else {

				if current == current.Parent.Left {
					current = current.Parent
					tree.RotateRight(current)
				}

				current.Parent.Color = Black
				current.Parent.Parent.Color = Red
				tree.RotateLeft(current.Parent.Parent)
			}
		}
	}

	// ensure that root is black
	tree.Root.Color = Black
}

// RotateLeft
// x is the axes of rotation, y is the node that will be replace x's position.
// we need to:
// 1. move y's left child to the x's right child
// 2. change y's parent to x's parent
// 3. change x's parent to y
func (tree *RBTree) RotateLeft(x *RBNode) {
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

func (tree *RBTree) RotateRight(y *RBNode) {
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

func (tree *RBTree) Rightmost(current *RBNode) *RBNode {
	for current.Right != nil {
		current = current.Right
	}

	return current
}

func (tree *RBTree) Leftmost(current *RBNode) *RBNode {
	for current.Left != nil {
		current = current.Left
	}

	return current
}

func (tree *RBTree) Successor(current *RBNode) *RBNode {
	if current.Right != nil {
		return tree.Leftmost(current.Right)
	}

	var newNode = current.Parent
	for newNode != nil && current == newNode.Right {
		current = newNode
		newNode = newNode.Parent
	}

	return newNode
}

func (tree *RBTree) Preorder(current *RBNode, cb func(n *RBNode)) {
	if current != nil {
		cb(current)
		tree.Preorder(current.Left, cb)
		tree.Preorder(current.Right, cb)
	}
}

func (tree *RBTree) Inorder(current *RBNode, cb func(n *RBNode)) {
	if current != nil {
		tree.Preorder(current.Left, cb)
		cb(current)
		tree.Preorder(current.Right, cb)
	}
}

func (tree *RBTree) Postorder(current *RBNode, cb func(n *RBNode)) {
	if current != nil {
		tree.Preorder(current.Left, cb)
		tree.Preorder(current.Right, cb)
		cb(current)
	}
}
