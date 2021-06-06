package types

import (
	"fmt"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

type RBTree struct {
	Root *RBNode
	neel *RBNode
	size int
}

func NewRBTree() *RBTree {
	var neel = &RBNode{
		Color: Black,
	}

	var root = neel
	root.Parent = neel

	return &RBTree{
		Root: root,
		neel: neel,
	}
}

func (tree *RBTree) Delete(key fixedpoint.Value) bool {
	var deleting = tree.Search(key)
	if deleting == nil {
		return false
	}

	// y = the node to be deleted
	// x (the child of the deleted node)
	var x, y *RBNode
	// fmt.Printf("neel = %p %+v\n", tree.neel, tree.neel)
	// fmt.Printf("deleting = %+v\n", deleting)

	// the deleting node has only one child, it's easy,
	// we just connect the child the parent of the deleting node
	if deleting.Left == tree.neel || deleting.Right == tree.neel {
		y = deleting
		// fmt.Printf("y = deleting = %+v\n", y)
	} else {
		// if both children are not NIL (neel), we need to find the successor
		// and copy the successor to the memory location of the deleting node.
		// since it's successor, it always has no child connecting to it.
		y = tree.Successor(deleting)
		// fmt.Printf("y = successor = %+v\n", y)
	}

	// y.Left or y.Right could be neel
	if y.Left != tree.neel {
		x = y.Left
	} else {
		x = y.Right
	}

	// fmt.Printf("x = %+v\n", y)
	x.Parent = y.Parent

	if y.Parent == tree.neel {
		tree.Root = x
	} else if y == y.Parent.Left {
		y.Parent.Left = x
	} else {
		y.Parent.Right = x
	}

	// copy the data from the successor to the memory location of the deleting node
	if y != deleting {
		deleting.Key = y.Key
		deleting.Value = y.Value
	}

	if y.Color == Black {
		tree.DeleteFixup(x)
	}

	tree.size--

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
	var y = tree.neel
	var x = tree.Root
	var node = &RBNode{
		Key:   key,
		Value: val,
		Color: Red,
	}

	for x != tree.neel {
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

	if y == tree.neel {
		tree.Root = node
	} else if node.Key < y.Key {
		y.Left = node
	} else {
		y.Right = node
	}

	node.Left = tree.neel
	node.Right = tree.neel
	node.Color = Red

	tree.InsertFixup(node)
}

func (tree *RBTree) Insert(key, val fixedpoint.Value) {
	var y = tree.neel
	var x = tree.Root
	var node = &RBNode{
		Key:   key,
		Value: val,
		Color: Red,
		Left:  tree.neel,
		Right: tree.neel,
	}

	for x != tree.neel {
		y = x

		if node.Key < x.Key {
			x = x.Left
		} else {
			x = x.Right
		}
	}

	node.Parent = y

	if y == tree.neel {
		tree.Root = node
	} else if node.Key < y.Key {
		y.Left = node
	} else {
		y.Right = node
	}

	node.Left = tree.neel
	node.Right = tree.neel
	node.Color = Red
	tree.size++

	tree.InsertFixup(node)
}

func (tree *RBTree) Search(key fixedpoint.Value) *RBNode {
	var current = tree.Root
	for current != tree.neel && key != current.Key {
		if key < current.Key {
			current = current.Left
		} else {
			current = current.Right
		}
	}

	if current == tree.neel {
		return nil
	}

	return current
}

func (tree *RBTree) Size() int {
	return tree.size
}

func (tree *RBTree) InsertFixup(current *RBNode) {
	// A red node can't have a red parent, we need to fix it up
	for current.Parent.Color == Red {
		if current.Parent == current.Parent.Parent.Left {
			uncle := current.Parent.Parent.Right
			if uncle.Color == Red {
				current.Parent.Color = Black
				uncle.Color = Black
				current.Parent.Parent.Color = Red
				current = current.Parent.Parent
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
				current = current.Parent.Parent
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

	if y.Left != tree.neel {
		y.Left.Parent = x
	}

	y.Parent = x.Parent

	if x.Parent == tree.neel {
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

	if x.Right != tree.neel {
		x.Right.Parent = y
	}

	x.Parent = y.Parent

	if y.Parent == tree.neel {
		tree.Root = x
	} else if y == y.Parent.Left {
		y.Parent.Left = x
	} else {
		y.Parent.Right = x
	}

	x.Right = y
	y.Parent = x
}

func (tree *RBTree) Rightmost() *RBNode {
	return tree.RightmostOf(tree.Root)
}

func (tree *RBTree) RightmostOf(current *RBNode) *RBNode {
	if current == tree.neel || current == nil {
		return nil
	}

	for current.Right != tree.neel && current.Right != nil {
		current = current.Right
	}

	if current == tree.neel {
		return nil
	}

	return current
}

func (tree *RBTree) Leftmost() *RBNode {
	return tree.LeftmostOf(tree.Root)
}

func (tree *RBTree) LeftmostOf(current *RBNode) *RBNode {
	if current == tree.neel || current == nil {
		return nil
	}

	for current.Left != tree.neel && current.Left != nil {
		current = current.Left
	}

	if current == tree.neel {
		return nil
	}

	return current
}

func (tree *RBTree) Successor(current *RBNode) *RBNode {
	if current.Right != tree.neel {
		return tree.LeftmostOf(current.Right)
	}

	var newNode = current.Parent
	for newNode != tree.neel && current == newNode.Right {
		current = newNode
		newNode = newNode.Parent
	}

	return newNode
}

func (tree *RBTree) Preorder(cb func(n *RBNode)) {
	tree.PreorderOf(tree.Root, cb)
}

func (tree *RBTree) PreorderOf(current *RBNode, cb func(n *RBNode)) {
	if current != tree.neel && current != nil {
		cb(current)
		tree.PreorderOf(current.Left, cb)
		tree.PreorderOf(current.Right, cb)
	}
}

// Inorder traverses the tree in ascending order
func (tree *RBTree) Inorder(cb func(n *RBNode) bool) {
	tree.InorderOf(tree.Root, cb)
}

func (tree *RBTree) InorderOf(current *RBNode, cb func(n *RBNode) bool) {
	if current != tree.neel && current != nil {
		tree.InorderOf(current.Left, cb)
		if !cb(current) {
			return
		}
		tree.InorderOf(current.Right, cb)
	}
}

// InorderReverse traverses the tree in descending order
func (tree *RBTree) InorderReverse(cb func(n *RBNode) bool) {
	tree.InorderReverseOf(tree.Root, cb)
}

func (tree *RBTree) InorderReverseOf(current *RBNode, cb func(n *RBNode) bool) {
	if current != tree.neel && current != nil {
		tree.InorderReverseOf(current.Right, cb)
		if !cb(current) {
			return
		}
		tree.InorderReverseOf(current.Left, cb)
	}
}

func (tree *RBTree) Postorder(cb func(n *RBNode) bool) {
	tree.PostorderOf(tree.Root, cb)
}

func (tree *RBTree) PostorderOf(current *RBNode, cb func(n *RBNode) bool) {
	if current != tree.neel && current != nil {
		tree.PostorderOf(current.Left, cb)
		tree.PostorderOf(current.Right, cb)
		if !cb(current) {
			return
		}
	}
}

func (tree *RBTree) copyNode(node, neel *RBNode) *RBNode {
	if node == tree.neel {
		return neel
	}

	newNode := *node
	newNode.Left = tree.copyNode(node.Left, neel)
	newNode.Right = tree.copyNode(node.Right, neel)
	return &newNode
}

func (tree *RBTree) CopyInorderReverse(limit int) *RBTree {
	cnt := 0
	newTree := NewRBTree()
	tree.InorderReverse(func(n *RBNode) bool {
		if cnt >= limit {
			return false
		}

		newTree.Insert(n.Key, n.Value)
		cnt++
		return true
	})
	return newTree
}

func (tree *RBTree) CopyInorder(limit int) *RBTree {
	cnt := 0
	newTree := NewRBTree()
	tree.Inorder(func(n *RBNode) bool {
		if cnt >= limit {
			return false
		}

		newTree.Insert(n.Key, n.Value)
		cnt++
		return true
	})

	return newTree
}

func (tree *RBTree) Print() {
	tree.Inorder(func(n *RBNode) bool {
		fmt.Printf("%f -> %f\n", n.Key.Float64(), n.Value.Float64())
		return true
	})
}

func (tree *RBTree) Copy() *RBTree {
	newTree := NewRBTree()
	newTree.Root = tree.copyNode(tree.Root, newTree.neel)
	return newTree
}
