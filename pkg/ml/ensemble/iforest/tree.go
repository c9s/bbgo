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

func (node *TreeNode) trackUsedFeatures(sample []float64, featureUsed []float64) []float64 {
	if node.IsLeaf() {
		return featureUsed
	}

	featureUsed[node.SplitIndex]++

	if sample[node.SplitIndex] < node.SplitValue {
		return node.Left.trackUsedFeatures(sample, featureUsed)
	} else {
		return node.Right.trackUsedFeatures(sample, featureUsed)
	}
}

func (node *TreeNode) FeatureImportance(sample []float64) []float64 {
	return node.trackUsedFeatures(sample, make([]float64, len(sample)))
}
