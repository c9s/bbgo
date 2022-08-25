package types

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/datatype/floats"
)

/*
python

import quantstats as qx
import pandas as pd

print(qx.stats.sortino(pd.Series([0.01, -0.03, 0.1, -0.02, 0.001]), 0.00, 0, False, False))
print(qx.stats.sortino(pd.Series([0.01, -0.03, 0.1, -0.02, 0.001]), 0.03, 252, False, False))
print(qx.stats.sortino(pd.Series([0.01, -0.03, 0.1, -0.02, 0.001]), 0.03, 252, True, False))
*/
func TestSortino(t *testing.T) {
	var a Series = &floats.Slice{0.01, -0.03, 0.1, -0.02, 0.001}
	output := Sortino(a, 0.03, 0, false, false)
	assert.InDelta(t, output, 0.75661, 0.0001)
	output = Sortino(a, 0.03, 252, false, false)
	assert.InDelta(t, output, 0.74597, 0.0001)
	output = Sortino(a, 0.03, 252, true, false)
	assert.InDelta(t, output, 11.84192, 0.0001)
}
