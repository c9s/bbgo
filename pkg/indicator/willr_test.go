package indicator

import (
	"encoding/json"
	"math"
	"testing"
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func TestWILLR_Update(t *testing.T) {
	highBytes := []byte(`[8279.0, 8282.0, 8280.0, 8280.0, 8284.0, 8284.0, 8280.0, 8282.0, 8284.0, 8289.0, 8288.0, 8285.0, 8284.0, 8287.0, 8286.0, 8294.0]`)
	lowBytes := []byte(`[8260.0, 8272.0, 8275.0, 8274.0, 8275.0, 8277.0, 8276.0, 8278.0, 8277.0, 8283.0, 8282.0, 8283.0, 8283.0, 8283.0, 8283.0, 8279.0]`)
	closeBytes := []byte(`[8262.0, 8273.0, 8279.0, 8279.0, 8275.0, 8282.0, 8278.0, 8279.0, 8281.0, 8285.0, 8287.0, 8284.0, 8283.0, 8283.0, 8285.0, 8286.0]`)

	var highs, lows, closes []float64
	_ = json.Unmarshal(highBytes, &highs)
	_ = json.Unmarshal(lowBytes, &lows)
	_ = json.Unmarshal(closeBytes, &closes)

	willr := &WILLR{
		IntervalWindow: types.IntervalWindow{Window: 14},
	}

	var updatedVal float64
	willr.OnUpdate(func(val float64) {
		updatedVal = val
	})

	now := time.Now()
	for i := range highs {
		kLine := types.KLine{
			High:    fixedpoint.NewFromFloat(highs[i]),
			Low:     fixedpoint.NewFromFloat(lows[i]),
			Close:   fixedpoint.NewFromFloat(closes[i]),
			EndTime: types.Time(now.Add(time.Duration(i) * time.Minute)),
		}
		willr.PushK(kLine)
	}

	if willr.Length() != len(highs) {
		t.Fatalf("expected length %d, got %d", len(highs), willr.Length())
	}

	// At index 13 (14th element, end of first 14-period window):
	// Highs from index 0 to 13: min low is 8260.0, max high is 8289.0
	// Close at index 13 is 8283.0
	// %R = -100 * (8289 - 8283) / (8289 - 8260) = -100 * 6 / 29 = -20.689655...
	expectedAt13 := -100.0 * (8289.0 - 8283.0) / (8289.0 - 8260.0)
	gotAt13 := willr.Values[13]
	if math.Abs(gotAt13-expectedAt13) > 1e-4 {
		t.Errorf("at index 13: got %f, want %f", gotAt13, expectedAt13)
	}

	lastVal := willr.Last(0)
	if lastVal != updatedVal {
		t.Errorf("expected Last(0) %f to match updatedVal %f", lastVal, updatedVal)
	}
	if willr.Index(0) != lastVal {
		t.Errorf("expected Index(0) %f to match Last(0) %f", willr.Index(0), lastVal)
	}
}

func TestWILLR_FlatMarket(t *testing.T) {
	willr := &WILLR{
		IntervalWindow: types.IntervalWindow{Window: 5},
	}

	// Flat prices: high == low == close
	for i := 0; i < 10; i++ {
		willr.Update(100.0, 100.0, 100.0)
	}

	for i, v := range willr.Values {
		if v != -50.0 {
			t.Errorf("expected -50.0 for flat market at %d, got %f", i, v)
		}
		if math.IsNaN(v) || math.IsInf(v, 0) {
			t.Fatalf("unexpected NaN or Inf at %d", i)
		}
	}
}
