package iforest

import "math"

const eulerGamma = 0.5772156649

// harmonicNumber returns the harmonic number for x using a known constant.
func harmonicNumber(x float64) float64 {
	return math.Log(x) + eulerGamma
}

// averagePathLength calculates expected path length for a given number of samples.
func averagePathLength(x float64) float64 {
	if x > 2 {
		return 2.0*harmonicNumber(x-1) - 2.0*(x-1)/x
	} else if x == 2 {
		return 1.0
	} else {
		return 0.0
	}
}

// pathLength traverses the given tree node and returns the path length for the vector.
func pathLength(vector []float64, node *TreeNode, currentPathLength int) float64 {
	if node.IsLeaf() {
		return float64(currentPathLength) + averagePathLength(float64(node.Size))
	}

	splitAttribute := node.SplitIndex
	splitValue := node.SplitValue
	if vector[splitAttribute] < splitValue {
		return pathLength(vector, node.Left, currentPathLength+1)
	} else {
		return pathLength(vector, node.Right, currentPathLength+1)
	}
}
