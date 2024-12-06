package iforest

import (
	"math/rand"
	"testing"

	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/mat"
)

func TestIsolationForest_Fit(t *testing.T) {
	// Generate synthetic data
	numSamples := 100
	numFeatures := 5
	data := make([]float64, numSamples*numFeatures)
	for i := range data {
		data[i] = rand.Float64()
	}
	samples := mat.NewDense(numSamples, numFeatures, data)

	// Initialize the IsolationForest
	iforest := New()

	// Fit the model
	iforest.Fit(samples)
}

func TestIsolationForest_Score(t *testing.T) {
	// Generate synthetic data
	numSamples := 100
	numFeatures := 5
	data := make([]float64, numSamples*numFeatures)
	for i := range data {
		data[i] = rand.Float64()
	}
	samples := mat.NewDense(numSamples, numFeatures, data)

	// Initialize the IsolationForest
	iforest := New()

	// Fit the model
	iforest.Fit(samples)

	// Score the samples
	scores := iforest.Score(samples)

	// Check the length of the scores slice
	if len(scores) != numSamples {
		t.Errorf("Expected %d scores, got %d", numSamples, len(scores))
	}

	// Optionally, check that scores are within the expected range
}

func TestIsolationForest_Predict(t *testing.T) {
	// Generate synthetic data
	numSamples := 100
	numFeatures := 5
	data := make([]float64, numSamples*numFeatures)
	for i := range data {
		data[i] = rand.Float64()
	}
	samples := mat.NewDense(numSamples, numFeatures, data)

	// Initialize the IsolationForest
	iforest := New()

	// Fit the model
	iforest.Fit(samples)

	// Predict anomalies
	predictions := iforest.Predict(samples)

	// Check the length of the predictions slice
	if len(predictions) != numSamples {
		t.Errorf("Expected %d predictions, got %d", numSamples, len(predictions))
	}

	// Verify that predictions are either 0 or 1
	for _, pred := range predictions {
		if pred != 0 && pred != 1 {
			t.Errorf("Invalid prediction value: %d", pred)
		}
	}
}

func TestIsolationForest_WithOutliers(t *testing.T) {
	// Generate normal data
	numSamples := 100
	numFeatures := 5
	data := make([]float64, numSamples*numFeatures)
	for i := range data {
		data[i] = rand.NormFloat64()
	}
	// Introduce outliers
	outlierData := []float64{1000, 1000, 1000, 1000, 1000}
	data = append(data, outlierData...)
	numSamples += 1
	samples := mat.NewDense(numSamples, numFeatures, data)

	// Initialize and fit the model
	iforest := New()
	iforest.Fit(samples)

	// Score the samples
	scores := iforest.Score(samples)

	// Check that the outlier has a higher anomaly score
	outlierScore := scores[numSamples-1]
	meanScore := floats.Sum(scores[:numSamples-1]) / float64(numSamples-1)
	if outlierScore <= meanScore {
		t.Errorf("Expected outlier score (%f) to be higher than mean score (%f)", outlierScore, meanScore)
	}
}

func TestIsolationForest_SampleSizeExceedsDataset(t *testing.T) {
	// Generate small synthetic data
	numSamples := 50
	numFeatures := 3
	data := make([]float64, numSamples*numFeatures)
	for i := range data {
		data[i] = rand.Float64()
	}
	samples := mat.NewDense(numSamples, numFeatures, data)

	// Set SampleSize larger than dataset size
	iforest := New()
	iforest.SampleSize = 100

	// Fit the model
	iforest.Fit(samples) // Should not panic or raise an error
}

func TestIsolationForest_RepeatedFit(t *testing.T) {
	// Generate initial data
	numSamples := 100
	numFeatures := 4
	data := make([]float64, numSamples*numFeatures)
	for i := range data {
		data[i] = rand.Float64()
	}
	samples := mat.NewDense(numSamples, numFeatures, data)

	// Initialize and fit the model
	iforest := New()
	iforest.Fit(samples)

	// Re-fit the model with new data
	newData := make([]float64, numSamples*numFeatures)
	for i := range newData {
		newData[i] = rand.Float64()
	}
	newSamples := mat.NewDense(numSamples, numFeatures, newData)

	iforest.Fit(newSamples) // Should overwrite the previous model without issues
}
