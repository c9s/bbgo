package indicatorv2

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/types"
)

/*
python:

import pandas as pd
from scipy.stats.mstats import gmean

data = pd.Series([1.1,1.2,1.3,1.4,1.5,1.6,1.7,1.8,1.9,1.1,1.2,1.3,1.4,1.5,1.6,1.7,1.8,1.9,1.1,1.2,1.3,1.4,1.5,1.6,1.7,1.8,1.9])
gmean(data[-5:])
gmean(data[-6:-1])
gmean(pd.concat(data[-4:], pd.Series([1.3])))
*/
func TestGMA(t *testing.T) {
	const delta = 1e-9
	data := []float64{1.1, 1.2, 1.3, 1.4, 1.5, 1.6, 1.7, 1.8, 1.9, 1.1, 1.2, 1.3, 1.4, 1.5, 1.6, 1.7, 1.8, 1.9, 1.1, 1.2, 1.3, 1.4, 1.5, 1.6, 1.7, 1.8, 1.9}

	source := types.NewFloat64Series()
	gma := GMA(source, 5)
	for i, d := range data {
		source.PushAndEmit(d)
		if i < 4 {
			assert.Equal(t, 0, gma.Length())
		}
	}

	assert.InDelta(t, 1.6940930229200213, gma.Last(0), delta)
	assert.InDelta(t, 1.5937204331251167, gma.Index(1), delta)

	source.PushAndEmit(1.3)
	assert.InDelta(t, 1.6462950504034335, gma.Last(0), delta)
	assert.Equal(t, 24, gma.Length())
}
