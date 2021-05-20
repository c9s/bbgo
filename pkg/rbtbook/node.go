package rbtbook

import "github.com/c9s/bbgo/pkg/fixedpoint"

type Color bool

const (
	Red   = Color(false)
	Black = Color(true)
)

/*
Node
A red node always has black children.
A black node may have red or black children
*/
type Node struct {
	Left, Right, Parent *Node
	Color               Color
	Key, Value          fixedpoint.Value
}
