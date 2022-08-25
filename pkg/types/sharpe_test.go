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

print(qx.stats.sharpe(pd.Series([0.01, 0.1, 0.001]), 0, 0, False, False))
print(qx.stats.sharpe(pd.Series([0.01, 0.1, 0.001]), 0, 252, False, False))
print(qx.stats.sharpe(pd.Series([0.01, 0.1, 0.001]), 0, 252, True, False))
*/
func TestSharpe(t *testing.T) {
	var a Series = &floats.Slice{0.01, 0.1, 0.001}
	output := Sharpe(a, 0, false, false)
	assert.InDelta(t, output, 0.67586, 0.0001)
	output = Sharpe(a, 252, false, false)
	assert.InDelta(t, output, 0.67586, 0.0001)
	output = Sharpe(a, 252, true, false)
	assert.InDelta(t, output, 10.7289, 0.0001)
}
