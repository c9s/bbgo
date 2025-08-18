package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSeriesBaseFuncWithPushData(t *testing.T) {
	series := NewFloat64Series(0.5, 1.0)
	series.Push(2.5)
	series.Push(3.0)
	series.Push(4.0)
	mean := series.Mean(5)
	assert.Equal(t, 2.2, mean)
	stdev := series.Stdev(5)
	assert.Equal(t, 1.2884098726725126, stdev)
}
