package iforest

import (
	"math"

	"golang.org/x/exp/rand"
	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/mat"
)

const (
	defaultNumTrees       = 100
	defaultSampleSize     = 256
	defaultScoreThreshold = 0.6
)

// Isolation Forest algorithm
// https://cs.nju.edu.cn/zhouzh/zhouzh.files/publication/icdm08b.pdf
// https://www.lamda.nju.edu.cn/publication/tkdd11.pdf
type IsolationForest struct {
	Forest         []*TreeNode
	ScoreThreshold float64 `json:"threshold"`
	NumTrees       int     `json:"numTrees"`
	SampleSize     int     `json:"sampleSize"`
	MaxDepth       int     `json:"heightLimit"`
}

func New() *IsolationForest {
	return &IsolationForest{
		ScoreThreshold: defaultScoreThreshold,
		NumTrees:       defaultNumTrees,
		SampleSize:     defaultSampleSize,
		MaxDepth:       int(math.Ceil(math.Log2(float64(defaultSampleSize)))),
	}
}

func (f *IsolationForest) BuildTree(samples *mat.Dense, depth int) *TreeNode {
	numSamples, numFeatures := samples.Dims()

	// Handle zero features by returning a leaf node
	if numFeatures == 0 {
		return &TreeNode{Size: numSamples}
	}

	if depth >= f.MaxDepth || numSamples <= 1 {
		return &TreeNode{Size: numSamples}
	}

	splitIndex := rand.Intn(numFeatures)
	col := mat.Col(nil, splitIndex, samples)
	min, max := floats.Min(col), floats.Max(col)
	splitValue := min + rand.Float64()*(max-min)

	leftData, rightData := []float64{}, []float64{}
	leftCount, rightCount := 0, 0
	for i := 0; i < numSamples; i++ {
		if samples.At(i, splitIndex) < splitValue {
			leftData = append(leftData, samples.RawRowView(i)...)
			leftCount++
		} else {
			rightData = append(rightData, samples.RawRowView(i)...)
			rightCount++
		}
	}

	return &TreeNode{
		Left:       f.BuildTree(mat.NewDense(leftCount, numFeatures, leftData), depth+1),
		Right:      f.BuildTree(mat.NewDense(rightCount, numFeatures, rightData), depth+1),
		SplitIndex: splitIndex,
		SplitValue: splitValue,
		Size:       numSamples,
	}
}

func (f *IsolationForest) Fit(samples *mat.Dense) {
	f.Forest = make([]*TreeNode, f.NumTrees)
	for i := 0; i < f.NumTrees; i++ {
		sampledData := sample(samples, f.SampleSize)
		f.Forest[i] = f.BuildTree(sampledData, 0)
	}
}

func (f *IsolationForest) Score(samples *mat.Dense) []float64 {
	numSamples, _ := samples.Dims()
	scores := make([]float64, numSamples)
	for i := 0; i < numSamples; i++ {
		s := 0.0
		for _, tree := range f.Forest {
			s += pathLength(samples.RawRowView(i), tree, 0)
		}
		scores[i] = math.Pow(2, -s/float64(f.NumTrees)/averagePathLength(float64(f.SampleSize)))
	}
	return scores
}

func (f *IsolationForest) Predict(samples *mat.Dense) []int {
	scores := f.Score(samples)
	predictions := make([]int, len(scores))
	for i, score := range scores {
		if score > f.ScoreThreshold {
			predictions[i] = 1
		} else {
			predictions[i] = 0
		}
	}
	return predictions
}

func (f *IsolationForest) FeatureImportance(sample []float64) []float64 {
	o := make([]float64, len(sample))

	for _, tree := range f.Forest {
		for i, c := range tree.FeatureImportance(sample) {
			o[i] += float64(c) / float64(f.NumTrees)
		}
	}

	return o
}
