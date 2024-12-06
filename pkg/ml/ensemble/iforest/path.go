package iforest

import "math"

const eulerGamma = 0.5772156649

func harmonicNumber(x float64) float64 {
	return math.Log(x) + eulerGamma
}

func averagePathLength(x float64) float64 {
	switch {
	case x > 2:
		return 2.0*harmonicNumber(x-1) - 2.0*(x-1)/x
	case x == 2:
		return 1.0
	default:
		return 0.0
	}
}

func pathLength(sample []float64, node *TreeNode, depth int) float64 {
	if node.IsLeaf() {
		return float64(depth) + averagePathLength(float64(node.Size))
	}

	if sample[node.SplitIndex] < node.SplitValue {
		return pathLength(sample, node.Left, depth+1)
	} else {
		return pathLength(sample, node.Right, depth+1)
	}
}
