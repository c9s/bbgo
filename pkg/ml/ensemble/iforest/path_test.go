package iforest

import (
	"math"
	"testing"
)

func TestHarmonicNumber(t *testing.T) {
	// Test when x > 0
	x := 5.0
	expected := math.Log(x) + eulerGamma
	result := harmonicNumber(x)
	if math.Abs(result-expected) > 1e-9 {
		t.Errorf("harmonicNumber(%v) = %v; want %v", x, result, expected)
	}
}

func TestAveragePathLength(t *testing.T) {
	// Test when x > 2
	x := 5.0
	expected := 2.0*harmonicNumber(x-1) - 2.0*(x-1)/x
	result := averagePathLength(x)
	if math.Abs(result-expected) > 1e-9 {
		t.Errorf("averagePathLength(%v) = %v; want %v", x, result, expected)
	}

	// Test when x == 2
	x = 2.0
	expected = 1.0
	result = averagePathLength(x)
	if result != expected {
		t.Errorf("averagePathLength(%v) = %v; want %v", x, result, expected)
	}

	// Test when x < 2
	x = 1.0
	expected = 0.0
	result = averagePathLength(x)
	if result != expected {
		t.Errorf("averagePathLength(%v) = %v; want %v", x, result, expected)
	}
}

func TestPathLength(t *testing.T) {
	// Leaf nodes
	leafLeft := &TreeNode{
		Size: 1,
	}

	leafRight := &TreeNode{
		Size: 1,
	}

	// Root node
	root := &TreeNode{
		Size:       2,
		SplitIndex: 0,
		SplitValue: 2.5,
		Left:       leafLeft,
		Right:      leafRight,
	}

	// Test sample that goes to the left child
	sampleLeft := []float64{2.0}
	resultLeft := pathLength(sampleLeft, root, 0)
	expectedLeft := 1.0 + averagePathLength(float64(leafLeft.Size))
	if resultLeft != expectedLeft {
		t.Errorf("pathLength(%v) = %v; want %v", sampleLeft, resultLeft, expectedLeft)
	}

	// Test sample that goes to the right child
	sampleRight := []float64{3.0}
	resultRight := pathLength(sampleRight, root, 0)
	expectedRight := 1.0 + averagePathLength(float64(leafRight.Size))
	if resultRight != expectedRight {
		t.Errorf("pathLength(%v) = %v; want %v", sampleRight, resultRight, expectedRight)
	}
}
