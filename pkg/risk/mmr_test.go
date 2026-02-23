package risk

import (
	"testing"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

// TestDefaultMaintenanceMarginRatio_Table ensures the returned MMR matches
// the expected tiered percentages across important boundaries.
func TestDefaultMaintenanceMarginRatio_Table(t *testing.T) {
	cases := []struct {
		name     string
		leverage fixedpoint.Value
		want     fixedpoint.Value
	}{
		{"below-3-default-9%", fixedpoint.NewFromFloat(2.0), fixedpoint.NewFromFloat(0.09)},
		{"just-below-3-default-9%", fixedpoint.NewFromFloat(2.99), fixedpoint.NewFromFloat(0.09)},
		{"at-3-10%", fixedpoint.NewFromFloat(3.0), fixedpoint.NewFromFloat(0.10)},
		{"mid-3to5-10%", fixedpoint.NewFromFloat(4.0), fixedpoint.NewFromFloat(0.10)},
		{"just-below-5-10%", fixedpoint.NewFromFloat(4.999), fixedpoint.NewFromFloat(0.10)},
		{"at-5-9%", fixedpoint.NewFromFloat(5.0), fixedpoint.NewFromFloat(0.09)},
		{"mid-5to10-9%", fixedpoint.NewFromFloat(9.0), fixedpoint.NewFromFloat(0.09)},
		{"just-below-10-9%", fixedpoint.NewFromFloat(9.999), fixedpoint.NewFromFloat(0.09)},
		{"at-10-5%", fixedpoint.NewFromFloat(10.0), fixedpoint.NewFromFloat(0.05)},
		{"above-10-5%", fixedpoint.NewFromFloat(20.0), fixedpoint.NewFromFloat(0.05)},
		{"far-above-10-5%", fixedpoint.NewFromFloat(100.0), fixedpoint.NewFromFloat(0.05)},
	}

	for _, tc := range cases {
		// capture range variable
		t.Run(tc.name, func(t *testing.T) {
			got := DefaultMaintenanceMarginRatio(tc.leverage)
			if got.Compare(tc.want) != 0 {
				t.Fatalf("unexpected MMR for leverage %s: got=%s want=%s", tc.leverage.String(), got.String(), tc.want.String())
			}
		})
	}
}
