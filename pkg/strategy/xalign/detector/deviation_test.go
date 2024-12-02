package detector

import (
	"testing"
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func TestBalanceDeviationDetector(t *testing.T) {
	// Initialize DeviationDetector for types.Balance
	detector := NewDeviationDetector(
		types.Balance{Currency: "BTC", NetAsset: fixedpoint.NewFromFloat(10.0)}, // Expected balance
		0.01,          // Tolerance percentage (1%)
		time.Minute*4, // Duration for sustained deviation
		func(b types.Balance) float64 {
			return b.Net().Float64() // Use Net() as the base for deviation detection
		},
	)

	now := time.Now()

	// Add a balance record within tolerance
	reset, sustainedDuration := detector.AddRecord(now, types.Balance{Currency: "BTC", NetAsset: fixedpoint.NewFromFloat(10.05)})
	if reset {
		t.Errorf("Expected no sustained deviation for value within tolerance")
	}
	if sustainedDuration != 0 {
		t.Errorf("Expected sustained duration to be 0 for value within tolerance, got %v", sustainedDuration)
	}

	// Add a balance record outside tolerance
	reset, sustainedDuration = detector.AddRecord(now.Add(2*time.Minute), types.Balance{Currency: "BTC", NetAsset: fixedpoint.NewFromFloat(11.0)})
	if reset {
		t.Errorf("Expected no sustained deviation initially")
	}
	if sustainedDuration != 0 {
		t.Errorf("Expected sustained duration to be 0 initially, got %v", sustainedDuration)
	}

	// Add another record exceeding duration
	reset, sustainedDuration = detector.AddRecord(now.Add(6*time.Minute), types.Balance{Currency: "BTC", NetAsset: fixedpoint.NewFromFloat(11.5)})
	if !reset {
		t.Errorf("Expected reset to be true")
	}
	if sustainedDuration != 4*time.Minute {
		t.Errorf("Expected sustained deviation to exceed threshold, got %v", sustainedDuration)
	}
}

func TestBalanceRecordTracking(t *testing.T) {
	// Initialize DeviationDetector for types.Balance
	detector := NewDeviationDetector(
		types.Balance{Currency: "BTC", NetAsset: fixedpoint.NewFromFloat(10.0)}, // Expected balance
		0.01,          // Tolerance percentage (1%)
		time.Minute*5, // Duration for sustained deviation
		func(b types.Balance) float64 {
			return b.Net().Float64()
		},
	)

	now := time.Now()

	// Add a balance record outside tolerance
	_, _ = detector.AddRecord(now, types.Balance{Currency: "BTC", NetAsset: fixedpoint.NewFromFloat(11.0)})

	// Check if record is being tracked
	records := detector.GetRecords()
	if len(records) != 1 {
		t.Errorf("Expected 1 record, got %d", len(records))
	}

	// Add another record
	_, _ = detector.AddRecord(now.Add(2*time.Minute), types.Balance{Currency: "BTC", NetAsset: fixedpoint.NewFromFloat(11.5)})
	records = detector.GetRecords()
	if len(records) != 2 {
		t.Errorf("Expected 2 records, got %d", len(records))
	}

	// Add a balance record within tolerance to reset
	_, _ = detector.AddRecord(now.Add(4*time.Minute), types.Balance{Currency: "BTC", NetAsset: fixedpoint.NewFromFloat(10.05)})
	records = detector.GetRecords()
	if len(records) != 0 {
		t.Errorf("Expected records to be cleared, got %d", len(records))
	}
}
