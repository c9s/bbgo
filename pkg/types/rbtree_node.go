package types

import "github.com/c9s/bbgo/pkg/fixedpoint"

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
	Left, Right, Parent *RBNode
	Color               Color
	Key, Value          fixedpoint.Value
}
