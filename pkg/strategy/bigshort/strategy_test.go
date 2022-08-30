package pivotshort

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_groupPivots(t *testing.T) {
	groups := groupPivots([]float64{1000, 1002, 1201, 1203, 1500}, 0.005)
	assert.Equal(t, [][]float64{[]float64{1500}, []float64{1203, 1201}, []float64{1002, 1000}}, groups)
}
