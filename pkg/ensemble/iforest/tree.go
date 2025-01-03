package iforest

type TreeNode struct {
	Left       *TreeNode
	Right      *TreeNode
	Size       int
	SplitIndex int
	SplitValue float64
}

// IsLeaf checks if the node is a leaf (no children).
func (t *TreeNode) IsLeaf() bool {
	return t.Left == nil && t.Right == nil
}

// traceSplitIndices traces the indices of the splits along the path of this tree node.
func (t *TreeNode) traceSplitIndices(sample []float64, indices []int) []int {
	if t.IsLeaf() {
		return indices
	}

	if sample[t.SplitIndex] < t.SplitValue {
		indices = append(indices, t.SplitIndex)
		return t.Left.traceSplitIndices(sample, indices)
	} else {
		indices = append(indices, t.SplitIndex)
		return t.Right.traceSplitIndices(sample, indices)
	}
}

// FeatureImportance calculates how many times each feature index is used during splits for the given sample.
func (t *TreeNode) FeatureImportance(sample []float64) []int {
	indices := t.traceSplitIndices(sample, []int{})
	importance := make([]int, len(sample))
	for _, index := range indices {
		importance[index]++
	}
	return importance
}
