package types

import "github.com/c9s/bbgo/pkg/fixedpoint"

// Color is the RB Tree color
type Color bool

const (
	Red   = Color(false)
	Black = Color(true)
)

/*
RBNode
A red node always has black children.
A black node may have red or black children
*/
type RBNode struct {
	left, right, parent *RBNode
	key, value          fixedpoint.Value
	color               Color
}

func NewNil() *RBNode {
	return &RBNode{color: Black}
}

func (node *RBNode) isNil() bool {
	if node == nil {
		return true
	}
	return node.color == Black && node.left == nil && node.right == nil
}
