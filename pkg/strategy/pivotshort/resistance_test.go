package pivotshort

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_findPossibleResistancePrices(t *testing.T) {
	prices := findPossibleResistancePrices(19000.0, 0.01, []float64{
		23020.0,
		23040.0,
		23060.0,

		24020.0,
		24040.0,
		24060.0,
	})
	assert.Equal(t, []float64{23035, 24040}, prices)


	prices = findPossibleResistancePrices(19000.0, 0.01, []float64{
		23020.0,
		23040.0,
		23060.0,
	})
	assert.Equal(t, []float64{23035}, prices)
}
