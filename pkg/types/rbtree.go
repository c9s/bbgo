package types

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

type RBTreeStats struct {
	alloc int64
	free  int64
}

type RBTree struct {
	Root     *RBNode
	size     int
	nodePool *sync.Pool // pool for RBNode allocation and reuse

	stats RBTreeStats
}

func NewRBTree() *RBTree {
	tree := &RBTree{
		nodePool: &sync.Pool{
			New: func() interface{} {
				return &RBNode{
					color: Black,
					key:   fixedpoint.Zero,
					value: fixedpoint.Zero,
				}
			},
		},
	}

	root := tree.newNilNode()
	root.parent = tree.newNilNode()
	tree.Root = root
	return tree
}

func (tree *RBTree) Delete(key fixedpoint.Value) bool {
	var deleting = tree.Search(key)
	if deleting == nil {
		return false
	}

	// x the child of the deleted node
	var x *RBNode

	// the deleting node has only one child, it's easy,
	// we just connect the child to the parent of the deleting node
	if deleting.left.isNil() || deleting.right.isNil() {
		wasBlack := deleting.color == Black

		// deleting.left or deleting.right could be neel
		if deleting.left.isNil() {
			x = deleting.right

			tree.clear(deleting.left)
			deleting.left = nil
		} else {
			x = deleting.left

			if deleting.right.isNil() {
				tree.clear(deleting.right)
				deleting.right = nil
			}
		}

		p := deleting.parent

		tree.transplant(deleting, x)
		tree.clear(deleting)

		if wasBlack {
			if err := tree.deleteFixup(x); err != nil {
				logrus.Errorf("delete fixup x = %+v error: %v", x, err)
				tree.printSubTreeGraph(p)
			}
		}

	} else {
		// if both children are not NIL (neel), we need to find the successor from the right subtree.
		// and copy the successor to the memory location of the deleting node.
		// since it's a successor, it always has no child connected to it.
		// here the right child is not nil, so it won't be nil.
		successor := tree.SuccessorOf(deleting)
		wasBlack := successor.color == Black
		deleting.key = successor.key
		deleting.value = successor.value

		if successor.left.isNil() {
			x = successor.right
			tree.clear(successor.left)
			successor.left = nil
		} else {
			x = successor.left

			if successor.right.isNil() {
				tree.clear(successor.right)
				successor.right = nil
			}
		}

		// if the successor has a right child, update its parent reference
		tree.transplant(successor, x)
		tree.clear(successor)

		if wasBlack {
			if err := tree.deleteFixup(x); err != nil {
				logrus.Errorf("delete fixup x = %+v error: %v", x, err)
				tree.printSubTreeGraph(deleting)
			}
		}
	}

	tree.size--

	return true
}

// transplant replaces sub-tree rooted at u with subtree rooted at v.
func (tree *RBTree) transplant(u, v *RBNode) {
	if v == nil {
		v = tree.newNilNode()
	}

	if u.parent == nil || u.parent.isNil() {
		tree.Root = v
		v.parent = tree.newNilNode()
		return
	} else if u == u.parent.left {
		u.parent.left = v
	} else if u == u.parent.right {
		u.parent.right = v
	}

	v.parent = u.parent
}

func (tree *RBTree) sanityCheck(n *RBNode) (err error) {
	tree.InorderOf(n, func(n *RBNode) bool {
		if n.left.isNil() {
			if n.left != nil {
				if n.left.color != Black {
					err = fmt.Errorf("nil left child is not black: %+v", n.left)
					return false
				}

				if n.left.left != nil {
					err = fmt.Errorf("nil left child's left is not nil: %+v", n.left.left)
					return false
				}

				if n.left.right != nil {
					err = fmt.Errorf("nil left child's right is not nil: %+v", n.left.right)
					return false
				}
			}
		} else if !(n.key.Compare(n.left.key) > 0) {
			err = fmt.Errorf("left child's key is not less than parent: left = %+v, parent = %+v", n.left.key, n.key)
			return false
		}

		if n.right.isNil() {
			if n.right != nil {
				if n.right.color != Black {
					err = fmt.Errorf("nil right child is not black: %+v", n.right)
					return false
				}

				if n.right.left != nil {
					err = fmt.Errorf("nil right child's left is not nil: %+v", n.right.left)
					return false
				}

				if n.right.right != nil {
					err = fmt.Errorf("nil right child's right is not nil: %+v", n.right.right)
					return false
				}
			}
		} else if !(n.key.Compare(n.right.key) < 0) {
			err = fmt.Errorf("right child's key is not greater than parent: right = %+v, parent = %+v", n.right.key, n.key)
			return false
		}

		return true
	})

	return err
}

func (tree *RBTree) deleteFixup(current *RBNode) error {
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
	return nil
}

func (tree *RBTree) Upsert(key, val fixedpoint.Value) {
	var y *RBNode = nil
	var x = tree.Root

	for !x.isNil() {
		y = x

		if x.key == key {
			// found node, skip insert and fix
			x.value = val
			return
		} else if key.Compare(x.key) < 0 {
			x = x.left
		} else {
			x = x.right
		}
	}

	node := tree.newValueNode(key, val, Red)

	if y == nil {
		if tree.Root != nil {
			tree.clear(tree.Root.parent)
			tree.clear(tree.Root)
		}
		// insert as the root node
		node.parent = tree.newNilNode()
		tree.Root = node
	} else {
		// insert as a child
		node.parent = y
		if node.key.Compare(y.key) < 0 {
			tree.clear(y.left)
			y.left = node
		} else {
			tree.clear(y.right)
			y.right = node
		}
	}

	tree.size++
	tree.InsertFixup(node)
}

func (tree *RBTree) Insert(key, val fixedpoint.Value) {
	var y *RBNode
	var x = tree.Root

	for !x.isNil() {
		y = x

		if key.Compare(x.key) < 0 {
			x = x.left
		} else {
			x = x.right
		}
	}

	node := tree.newValueNode(key, val, Red)

	if y == nil {
		if tree.Root != nil {
			tree.clear(tree.Root.parent)
			tree.clear(tree.Root)
		}

		node.parent = tree.newNilNode()
		tree.Root = node
	} else {
		node.parent = y
		if node.key.Compare(y.key) < 0 {
			tree.clear(y.left)
			y.left = node
		} else {
			tree.clear(y.right)
			y.right = node
		}
	}

	tree.size++
	tree.InsertFixup(node)
}

func (tree *RBTree) Search(key fixedpoint.Value) *RBNode {
	var current = tree.Root
	for !current.isNil() && key.Compare(current.key) != 0 {
		if key.Compare(current.key) < 0 {
			current = current.left
		} else {
			current = current.right
		}
	}

	if current.isNil() {
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

	if !y.left.isNil() {
		y.left.parent = x
	}

	y.parent = x.parent

	if x.parent.isNil() {
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

	if !x.right.isNil() {
		if x.right == nil {
			logrus.Panicf("x.right is nil: node = %+v, left = %+v, right = %+v, parent = %+v", x, x.left, x.right, x.parent)
		}

		x.right.parent = y
	}

	x.parent = y.parent

	if y.parent.isNil() {
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
	if current == nil || current.isNil() {
		return nil
	}

	for !current.right.isNil() {
		current = current.right
	}

	return current
}

func (tree *RBTree) Leftmost() *RBNode {
	return tree.LeftmostOf(tree.Root)
}

func (tree *RBTree) LeftmostOf(current *RBNode) *RBNode {
	if current == nil || current.isNil() {
		return nil
	}

	for !current.left.isNil() {
		current = current.left
	}

	return current
}

func (tree *RBTree) SuccessorOf(current *RBNode) *RBNode {
	if !current.right.isNil() {
		return tree.LeftmostOf(current.right)
	}

	// otherwise walk up until we find a node that is a left child of its parent
	var suc = current.parent
	for !suc.isNil() && current == suc.right {
		current = suc
		suc = suc.parent
	}

	return suc
}

func (tree *RBTree) Preorder(cb func(n *RBNode)) {
	tree.PreorderOf(tree.Root, cb)
}

func (tree *RBTree) PreorderOf(current *RBNode, cb func(n *RBNode)) {
	if current != nil && !current.isNil() {
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
	if current != nil && !current.isNil() {
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
	if current != nil && !current.isNil() {
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
	if current != nil && !current.isNil() {
		tree.PostorderOf(current.left, cb)
		tree.PostorderOf(current.right, cb)
		if !cb(current) {
			return
		}
	}
}

func (tree *RBTree) CopyInorderReverse(limit int) *RBTree {
	newTree := NewRBTree()
	if limit == 0 {
		tree.InorderReverse(copyNodeFast(newTree))
		return newTree
	}

	tree.InorderReverse(copyNodeLimit(newTree, limit))
	return newTree
}

func (tree *RBTree) CopyInorder(limit int) *RBTree {
	newTree := NewRBTree()
	if limit == 0 {
		tree.Inorder(copyNodeFast(newTree))
		return newTree
	}

	tree.Inorder(copyNodeLimit(newTree, limit))
	return newTree
}

func (tree *RBTree) Print() {
	tree.Inorder(func(n *RBNode) bool {
		fmt.Printf("%v -> %v\n", n.key, n.value)
		return true
	})
}

// newNilNode allocates a nil RBNode from the pool
func (tree *RBTree) newNilNode() *RBNode {
	n := tree.nodePool.Get().(*RBNode)
	n.left = nil
	n.right = nil
	n.parent = nil
	n.key = fixedpoint.Zero
	n.value = fixedpoint.Zero
	n.color = Black

	atomic.AddInt64(&tree.stats.alloc, 1)
	return n
}

func (tree *RBTree) newValueNode(key, value fixedpoint.Value, color Color) *RBNode {
	n := tree.nodePool.Get().(*RBNode)
	n.left = tree.newNilNode()
	n.right = tree.newNilNode()
	n.parent = nil
	n.key = key
	n.value = value
	n.color = color
	atomic.AddInt64(&tree.stats.alloc, 1)
	return n
}

// clear releases the node to the pool
func (tree *RBTree) clear(n *RBNode) {
	if n == nil {
		return
	}

	if n.left != nil {
		if n.left.isNil() && n.left.parent == n {
			tree.clear(n.left)
		}

		n.left = nil
	}

	if n.right != nil {
		if n.right.isNil() && n.right.parent == n {
			tree.clear(n.right)
		}

		n.right = nil
	}

	n.parent = nil
	tree.nodePool.Put(n)
	atomic.AddInt64(&tree.stats.free, 1)
}

func copyNodeFast(newTree *RBTree) func(n *RBNode) bool {
	return func(n *RBNode) bool {
		newTree.Insert(n.key, n.value)
		return true
	}
}

func copyNodeLimit(newTree *RBTree, limit int) func(n *RBNode) bool {
	cnt := 0
	return func(n *RBNode) bool {
		if limit > 0 && cnt >= limit {
			return false
		}

		newTree.Insert(n.key, n.value)
		cnt++
		return true
	}
}

// PrintSubTreeGraph prints the graph of a sub-tree starting from the given node.
func (tree *RBTree) printSubTreeGraph(node *RBNode) {
	if node == nil {
		fmt.Println("<empty>")
		return
	}
	printSubTree(node, "", true)
}

func printSubTree(node *RBNode, prefix string, isTail bool) {
	if node == nil {
		return
	}

	color := "B"
	if node.color == Red {
		color = "R"
	}

	fmt.Printf("%s%s── %d(%s)\n", prefix, getBranch(isTail), node.key, color)

	newPrefix := prefix + getIndent(isTail)
	if node.left != nil || node.right != nil {
		printSubTree(node.right, newPrefix, false)
		printSubTree(node.left, newPrefix, true)
	}
}

func getBranch(isTail bool) string {
	if isTail {
		return "└"
	}
	return "├"
}

func getIndent(isTail bool) string {
	if isTail {
		return "   "
	}
	return "│  "
}
