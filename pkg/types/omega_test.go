package types

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/datatype/floats"
)

func TestOmega(t *testing.T) {
	var a Series = &floats.Slice{0.08, 0.09, 0.07, 0.15, 0.02, 0.03, 0.04, 0.05, 0.06, 0.01}
	output := Omega(a)
	assert.InDelta(t, output, 1, 0.0001)
}

func TestOmegaExplicitThreshold(t *testing.T) {
	for _, tc := range []struct {
		name      string
		returns   floats.Slice
		threshold float64
		want      float64
	}{
		{"more gains than losses", floats.Slice{-0.1, 0.2}, 0, 2},
		{"more losses than gains", floats.Slice{-0.2, 0.1}, 0, 0.5},
		{"nonzero threshold", floats.Slice{0, 0.3}, 0.1, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.InDelta(t, tc.want, Omega(&tc.returns, tc.threshold), 0.0001)
		})
	}
}
