package detector

import (
	"testing"
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func TestDeviationWithTolerancePercentage(t *testing.T) {
	// Initialize DeviationDetector with float64 values
	detector := NewDeviationDetector(
		100.0,         // Expected value
		0.01,          // Tolerance percentage (1%)
		time.Minute*5, // Duration for sustained deviation
		nil,           // Use default conversion for float64
	)

	// Define timestamps for testing
	t1 := time.Date(2023, 1, 1, 10, 0, 0, 0, time.UTC)
	t2 := t1.Add(4 * time.Minute)
	t3 := t1.Add(6 * time.Minute)

	// Add a record within tolerance (1%)
	reset, sustainedDuration := detector.AddRecord("BTC", 101.0, t1)
	if len(detector.GetRecords("BTC")) != 0 || reset {
		t.Errorf("Expected records to reset when value is within tolerance")
	}

	// Add a record outside tolerance
	reset, sustainedDuration = detector.AddRecord("BTC", 110.0, t1)
	if reset || sustainedDuration != 0 {
		t.Errorf("Expected no sustained deviation initially")
	}

	// Add another record within duration
	reset, sustainedDuration = detector.AddRecord("BTC", 112.0, t2)
	if reset || sustainedDuration != 4*time.Minute {
		t.Errorf("Expected sustained deviation to be less than threshold")
	}

	// Add another record exceeding duration
	reset, sustainedDuration = detector.AddRecord("BTC", 112.0, t3)
	if !reset || sustainedDuration != 6*time.Minute {
		t.Errorf("Expected sustained deviation to exceed threshold")
	}
}

func TestDefaultToFloat(t *testing.T) {
	// Test default toFloat64 for float64 type
	detector := NewDeviationDetector(
		100.0,         // Expected value
		0.01,          // Tolerance percentage (1%)
		time.Minute*5, // Duration for sustained deviation
		nil,           // Use default conversion for float64
	)

	// Define timestamps for testing
	t1 := time.Now()

	// Add a record within tolerance
	reset, _ := detector.AddRecord("BTC", 100.5, t1)
	if reset {
		t.Errorf("Expected no sustained deviation for value within tolerance")
	}

	// Add a record outside tolerance
	reset, _ = detector.AddRecord("BTC", 105.0, t1.Add(2*time.Minute))
	if reset {
		t.Errorf("Expected no sustained deviation initially")
	}
}

func TestBalanceDeviationDetector(t *testing.T) {
	detector := NewDeviationDetector(
		types.Balance{Currency: "BTC", NetAsset: fixedpoint.NewFromFloat(10.0)}, // Expected value
		0.01,          // Tolerance (1%)
		time.Minute*5, // Duration for sustained deviation
		func(b types.Balance) float64 {
			return b.Net().Float64()
		},
	)

	now := time.Now()

	// Add a balance record within tolerance
	reset, _ := detector.AddRecord("BTC", types.Balance{Currency: "BTC", NetAsset: fixedpoint.NewFromFloat(10.05)}, now)
	if reset {
		t.Errorf("Expected no sustained deviation for value within tolerance")
	}

	// Add a balance record outside tolerance
	reset, _ = detector.AddRecord("BTC", types.Balance{Currency: "BTC", NetAsset: fixedpoint.NewFromFloat(9.5)}, now.Add(2*time.Minute))
	if reset {
		t.Errorf("Expected no sustained deviation initially")
	}

	// Add another record exceeding duration
	reset, sustainedDuration := detector.AddRecord("BTC", types.Balance{Currency: "BTC", NetAsset: fixedpoint.NewFromFloat(9.0)}, now.Add(6*time.Minute))
	if !reset || sustainedDuration != 6*time.Minute {
		t.Errorf("Expected sustained deviation to exceed threshold, got %v", sustainedDuration)
	}
}
