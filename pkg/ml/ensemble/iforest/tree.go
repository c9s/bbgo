package iforest

type TreeNode struct {
	Left       *TreeNode
	Right      *TreeNode
	SplitIndex int
	SplitValue float64
	Size       int
}

func (node *TreeNode) IsLeaf() bool {
	return node.Left == nil && node.Right == nil
}
