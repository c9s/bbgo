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
