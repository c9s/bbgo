package iforest

import (
	"math"
	"math/rand"
	"sync"
)

const (
	defaultNumTrees       = 100
	defaultSampleSize     = 256
	defaultScoreThreshold = 0.6
	defaultDetectionType  = DetectionTypeThreshold
	offset                = 0.5
)

type DetectionType string

const (
	DetectionTypeThreshold  DetectionType = "threshold"
	DetectionTypeProportion DetectionType = "proportion"
)

type Options struct {
	// The method used for anomaly detection
	DetectionType DetectionType `json:"detectionType"`

	// The anomaly score threshold
	Threshold float64 `json:"threshold"`

	// The proportion of outliers in the dataset
	Proportion float64 `json:"proportion"`

	// The number of trees to build in the forest
	NumTrees int `json:"numTrees"`

	// The sample size for each isolation tree
	SampleSize int `json:"sampleSize"`

	// The maximum depth of each isolation tree
	MaxDepth int `json:"maxDepth"`
}

// SetDefaultValues applies default settings to unspecified fields
func (o *Options) SetDefaultValues() {
	if o.DetectionType == "" {
		o.DetectionType = defaultDetectionType
	}

	if o.Threshold == 0 {
		o.Threshold = defaultScoreThreshold
	}

	if o.NumTrees == 0 {
		o.NumTrees = defaultNumTrees
	}

	if o.SampleSize == 0 {
		o.SampleSize = defaultSampleSize
	}

	if o.MaxDepth == 0 {
		o.MaxDepth = int(math.Ceil(math.Log2(float64(o.SampleSize))))
	}
}

// IsolationForest orchestrates anomaly detection using isolation trees
type IsolationForest struct {
	*Options

	Trees []*TreeNode
}

// New creates an IsolationForest with default options.
func New() *IsolationForest {
	options := &Options{}
	options.SetDefaultValues()
	return &IsolationForest{Options: options}
}

// NewWithOptions creates an IsolationForest with the specified options.
func NewWithOptions(options Options) *IsolationForest {
	options.SetDefaultValues()
	return &IsolationForest{Options: &options}
}

// Fit constructs isolation trees from a given dataset
func (f *IsolationForest) Fit(samples [][]float64) {
	wg := sync.WaitGroup{}
	wg.Add(f.NumTrees)

	f.Trees = make([]*TreeNode, f.NumTrees)
	for i := 0; i < f.NumTrees; i++ {
		sampled := SampleRows(samples, f.SampleSize)
		go func(index int) {
			defer wg.Done()
			tree := f.BuildTree(sampled, 0)
			f.Trees[index] = tree
		}(i)
	}
	wg.Wait()
}

// BuildTree recursively partitions samples to isolate outliers
func (f *IsolationForest) BuildTree(samples [][]float64, depth int) *TreeNode {
	numSamples := len(samples)
	if numSamples == 0 {
		return &TreeNode{}
	}
	numFeatures := len(samples[0])
	if depth >= f.MaxDepth || numSamples <= 1 {
		return &TreeNode{Size: numSamples}
	}

	splitIndex := rand.Intn(numFeatures)
	column := Column(samples, splitIndex)
	minValue, maxValue := MinMax(column)
	splitValue := rand.Float64()*(maxValue-minValue) + minValue

	leftSamples := make([][]float64, 0)
	rightSamples := make([][]float64, 0)
	for _, sample := range samples {
		if sample[splitIndex] < splitValue {
			leftSamples = append(leftSamples, sample)
		} else {
			rightSamples = append(rightSamples, sample)
		}
	}

	return &TreeNode{
		Left:       f.BuildTree(leftSamples, depth+1),
		Right:      f.BuildTree(rightSamples, depth+1),
		SplitIndex: splitIndex,
		SplitValue: splitValue,
	}
}

// Score computes anomaly scores for each sample
func (f *IsolationForest) Score(samples [][]float64) []float64 {
	scores := make([]float64, len(samples))
	for i, sample := range samples {
		score := 0.0
		for _, tree := range f.Trees {
			score += pathLength(sample, tree, 0)
		}
		scores[i] = math.Pow(2.0, -score/float64(len(f.Trees))/averagePathLength(float64(f.SampleSize)))
	}
	return scores
}

// Predict labels samples as outliers (1) or normal (0) based on the detection type
func (f *IsolationForest) Predict(samples [][]float64) []int {
	predictions := make([]int, len(samples))
	scores := f.Score(samples)

	var threshold float64
	switch f.DetectionType {
	case DetectionTypeThreshold:
		threshold = f.Threshold
	case DetectionTypeProportion:
		threshold = Quantile(f.Score(samples), 1-f.Proportion)
	default:
		panic("Invalid detection type")
	}

	for i, score := range scores {
		if score >= threshold {
			predictions[i] = 1
		} else {
			predictions[i] = 0
		}
	}

	return predictions
}

// FeatureImportance computes an importance score for each feature
func (f *IsolationForest) FeatureImportance(sample []float64) []int {
	importance := make([]int, len(sample))
	for _, tree := range f.Trees {
		for i, value := range tree.FeatureImportance(sample) {
			importance[i] += value
		}
	}
	return importance
}
