package bst

// Comparable interface.
type Comparable interface {
	// Compare with other value. Returns -1 if less than, 0 if
	// equals, and 1 if greather than the other value.
	Compare(other Comparable) int
}

// Compares first and second values. The given values must be
// numeric or must implement Comparable interface.
//
// Returns -1 if less than, 0 if equals, 1 if greather than.
func Compare(first, second interface{}) int {
	if _, ok := first.(Comparable); ok {
		return first.(Comparable).Compare(second.(Comparable))
	}

	switch first.(type) {

	case float64:
		return compareFloat64(first.(float64), second.(float64))

	case int64:
		return compareInt64(first.(int64), second.(int64))
	}

	panic("not comparable")
}

func compareFloat64(first, second float64) int {
	if first < second {
		return -1
	}

	if first > second {
		return 1
	}

	return 0
}

func compareInt64(first, second int64) int {
	if first < second {
		return -1
	}

	if first > second {
		return 1
	}

	return 0
}

// Remove node.
func (t *Tree) removeNode(parent, node *Node) {
	if node.left != nil && node.right != nil {
		min, minParent := minNode(node.right)
		if minParent == nil {
			minParent = node
		}

		t.removeNode(minParent, min)
		node.value = min.value
	} else {
		var child *Node
		if node.left != nil {
			child = node.left
		} else {
			child = node.right
		}

		if node == t.root {
			t.root = child
		} else if parent.left == node {
			parent.left = child
		} else {
			parent.right = child
		}
	}
}

// Min node. Returns min node and its parent.
func minNode(root *Node) (*Node, *Node) {
	if root == nil {
		return nil, nil
	}

	var parent *Node
	node := root

	for node.left != nil {
		parent = node
		node = node.left
	}

	return node, parent
}

// Max node. Returns max node and its parent.
func maxNode(root *Node) (*Node, *Node) {
	if root == nil {
		return nil, nil
	}

	var parent *Node
	node := root

	for node.right != nil {
		parent = node
		node = node.right
	}

	return node, parent
}
