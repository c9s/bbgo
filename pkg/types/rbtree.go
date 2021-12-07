package types

import (
	"fmt"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

type RBTree struct {
	Root *RBNode
	size int
}

var neel = &RBNode{ color: Black }

func NewRBTree() *RBTree {
	var root = neel
	root.parent = neel
	return &RBTree{
		Root: root,
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
	// fmt.Printf("neel = %p %+v\n", neel, neel)
	// fmt.Printf("deleting = %+v\n", deleting)

	// the deleting node has only one child, it's easy,
	// we just connect the child the parent of the deleting node
	if deleting.left == neel || deleting.right == neel {
		y = deleting
		// fmt.Printf("y = deleting = %+v\n", y)
	} else {
		// if both children are not NIL (neel), we need to find the successor
		// and copy the successor to the memory location of the deleting node.
		// since it's successor, it always has no child connecting to it.
		y = tree.Successor(deleting)
		// fmt.Printf("y = successor = %+v\n", y)
	}

	// y.left or y.right could be neel
	if y.left != neel {
		x = y.left
	} else {
		x = y.right
	}

	// fmt.Printf("x = %+v\n", y)
	x.parent = y.parent

	if y.parent == neel {
		tree.Root = x
	} else if y == y.parent.left {
		y.parent.left = x
	} else {
		y.parent.right = x
	}

	// copy the data from the successor to the memory location of the deleting node
	if y != deleting {
		deleting.key = y.key
		deleting.value = y.value
	}

	if y.color == Black {
		tree.DeleteFixup(x)
	}

	tree.size--

	return true
}

func (tree *RBTree) DeleteFixup(current *RBNode) {
	for current != tree.Root && current.color == Black {
		if current == current.parent.left {
			sibling := current.parent.right
			if sibling.color == Red {
				sibling.color = Black
				current.parent.color = Red
				tree.RotateLeft(current.parent)
				sibling = current.parent.right
			}

			// if both are black nodes
			if sibling.left.color == Black && sibling.right.color == Black {
				sibling.color = Red
				current = current.parent
			} else {
				// only one of the child is black
				if sibling.right.color == Black {
					sibling.left.color = Black
					sibling.color = Red
					tree.RotateRight(sibling)
					sibling = current.parent.right
				}

				sibling.color = current.parent.color
				current.parent.color = Black
				sibling.right.color = Black
				tree.RotateLeft(current.parent)
				current = tree.Root
			}
		} else { // if current is right child
			sibling := current.parent.left
			if sibling.color == Red {
				sibling.color = Black
				current.parent.color = Red
				tree.RotateRight(current.parent)
				sibling = current.parent.left
			}

			if sibling.left.color == Black && sibling.right.color == Black {
				sibling.color = Red
				current = current.parent
			} else { // if only one of child is Black

				// the left child of sibling is black, and right child is red
				if sibling.left.color == Black {
					sibling.right.color = Black
					sibling.color = Red
					tree.RotateLeft(sibling)
					sibling = current.parent.left
				}

				sibling.color = current.parent.color
				current.parent.color = Black
				sibling.left.color = Black
				tree.RotateRight(current.parent)
				current = tree.Root
			}
		}
	}

	current.color = Black
}

func (tree *RBTree) Upsert(key, val fixedpoint.Value) {
	var y = neel
	var x = tree.Root
	var node = &RBNode{
		key:   key,
		value: val,
		color: Red,
		left:  neel,
		right: neel,
	}

	for x != neel {
		y = x

		if node.key == x.key {
			// found node, skip insert and fix
			x.value = val
			return
		} else if node.key < x.key {
			x = x.left
		} else {
			x = x.right
		}
	}

	node.parent = y

	if y == neel {
		tree.Root = node
	} else if node.key < y.key {
		y.left = node
	} else {
		y.right = node
	}

	tree.InsertFixup(node)
}

func (tree *RBTree) Insert(key, val fixedpoint.Value) {
	var y = neel
	var x = tree.Root
	var node = &RBNode{
		key:   key,
		value: val,
		color: Red,
		left:  neel,
		right: neel,
	}

	for x != neel {
		y = x

		if node.key < x.key {
			x = x.left
		} else {
			x = x.right
		}
	}

	node.parent = y

	if y == neel {
		tree.Root = node
	} else if node.key < y.key {
		y.left = node
	} else {
		y.right = node
	}

	tree.size++
	tree.InsertFixup(node)
}

func (tree *RBTree) Search(key fixedpoint.Value) *RBNode {
	var current = tree.Root
	for current != neel && key != current.key {
		if key < current.key {
			current = current.left
		} else {
			current = current.right
		}
	}

	if current == neel {
		return nil
	}

	return current
}

func (tree *RBTree) Size() int {
	return tree.size
}

func (tree *RBTree) InsertFixup(current *RBNode) {
	// A red node can't have a red parent, we need to fix it up
	for current.parent.color == Red {
		if current.parent == current.parent.parent.left {
			uncle := current.parent.parent.right
			if uncle.color == Red {
				current.parent.color = Black
				uncle.color = Black
				current.parent.parent.color = Red
				current = current.parent.parent
			} else { // if uncle is black
				if current == current.parent.right {
					current = current.parent
					tree.RotateLeft(current)
				}

				current.parent.color = Black
				current.parent.parent.color = Red
				tree.RotateRight(current.parent.parent)
			}
		} else {
			uncle := current.parent.parent.left
			if uncle.color == Red {
				current.parent.color = Black
				uncle.color = Black
				current.parent.parent.color = Red
				current = current.parent.parent
			} else {
				if current == current.parent.left {
					current = current.parent
					tree.RotateRight(current)
				}

				current.parent.color = Black
				current.parent.parent.color = Red
				tree.RotateLeft(current.parent.parent)
			}
		}
	}

	// ensure that root is black
	tree.Root.color = Black
}

// RotateLeft
// x is the axes of rotation, y is the node that will be replace x's position.
// we need to:
// 1. move y's left child to the x's right child
// 2. change y's parent to x's parent
// 3. change x's parent to y
func (tree *RBTree) RotateLeft(x *RBNode) {
	var y = x.right
	x.right = y.left

	if y.left != neel {
		y.left.parent = x
	}

	y.parent = x.parent

	if x.parent == neel {
		tree.Root = y
	} else if x == x.parent.left {
		x.parent.left = y
	} else {
		x.parent.right = y
	}

	y.left = x
	x.parent = y
}

func (tree *RBTree) RotateRight(y *RBNode) {
	x := y.left
	y.left = x.right

	if x.right != neel {
		x.right.parent = y
	}

	x.parent = y.parent

	if y.parent == neel {
		tree.Root = x
	} else if y == y.parent.left {
		y.parent.left = x
	} else {
		y.parent.right = x
	}

	x.right = y
	y.parent = x
}

func (tree *RBTree) Rightmost() *RBNode {
	return tree.RightmostOf(tree.Root)
}

func (tree *RBTree) RightmostOf(current *RBNode) *RBNode {
	if current == neel || current == nil {
		return nil
	}

	for current.right != neel && current.right != nil {
		current = current.right
	}

	if current == neel {
		return nil
	}

	return current
}

func (tree *RBTree) Leftmost() *RBNode {
	return tree.LeftmostOf(tree.Root)
}

func (tree *RBTree) LeftmostOf(current *RBNode) *RBNode {
	if current == neel || current == nil {
		return nil
	}

	for current.left != neel && current.left != nil {
		current = current.left
	}

	if current == neel {
		return nil
	}

	return current
}

func (tree *RBTree) Successor(current *RBNode) *RBNode {
	if current.right != neel {
		return tree.LeftmostOf(current.right)
	}

	var newNode = current.parent
	for newNode != neel && current == newNode.right {
		current = newNode
		newNode = newNode.parent
	}

	return newNode
}

func (tree *RBTree) Preorder(cb func(n *RBNode)) {
	tree.PreorderOf(tree.Root, cb)
}

func (tree *RBTree) PreorderOf(current *RBNode, cb func(n *RBNode)) {
	if current != neel && current != nil {
		cb(current)
		tree.PreorderOf(current.left, cb)
		tree.PreorderOf(current.right, cb)
	}
}

// Inorder traverses the tree in ascending order
func (tree *RBTree) Inorder(cb func(n *RBNode) bool) {
	tree.InorderOf(tree.Root, cb)
}

func (tree *RBTree) InorderOf(current *RBNode, cb func(n *RBNode) bool) {
	if current != neel && current != nil {
		tree.InorderOf(current.left, cb)
		if !cb(current) {
			return
		}
		tree.InorderOf(current.right, cb)
	}
}

// InorderReverse traverses the tree in descending order
func (tree *RBTree) InorderReverse(cb func(n *RBNode) bool) {
	tree.InorderReverseOf(tree.Root, cb)
}

func (tree *RBTree) InorderReverseOf(current *RBNode, cb func(n *RBNode) bool) {
	if current != neel && current != nil {
		tree.InorderReverseOf(current.right, cb)
		if !cb(current) {
			return
		}
		tree.InorderReverseOf(current.left, cb)
	}
}

func (tree *RBTree) Postorder(cb func(n *RBNode) bool) {
	tree.PostorderOf(tree.Root, cb)
}

func (tree *RBTree) PostorderOf(current *RBNode, cb func(n *RBNode) bool) {
	if current != neel && current != nil {
		tree.PostorderOf(current.left, cb)
		tree.PostorderOf(current.right, cb)
		if !cb(current) {
			return
		}
	}
}

func (tree *RBTree) copyNode(node *RBNode) *RBNode {
	if node == neel {
		return neel
	}

	newNode := *node
	newNode.left = tree.copyNode(node.left)
	newNode.right = tree.copyNode(node.right)
	return &newNode
}

func (tree *RBTree) CopyInorderReverse(limit int) *RBTree {
	cnt := 0
	newTree := NewRBTree()
	tree.InorderReverse(func(n *RBNode) bool {
		if cnt >= limit {
			return false
		}

		newTree.Insert(n.key, n.value)
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

		newTree.Insert(n.key, n.value)
		cnt++
		return true
	})

	return newTree
}

func (tree *RBTree) Print() {
	tree.Inorder(func(n *RBNode) bool {
		fmt.Printf("%f -> %f\n", n.key.Float64(), n.value.Float64())
		return true
	})
}

func (tree *RBTree) Copy() *RBTree {
	newTree := NewRBTree()
	newTree.Root = tree.copyNode(tree.Root)
	return newTree
}
