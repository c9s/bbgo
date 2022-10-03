package drift

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_TrailingCheckLong(t *testing.T) {
	s := &Strategy{}
	s.highestPrice = 30.
	s.buyPrice = 30.
	s.TrailingActivationRatio = []float64{0.002, 0.01}
	s.TrailingCallbackRate = []float64{0.0008, 0.0016}
	assert.False(t, s.trailingCheck(31., "long"))
	assert.False(t, s.trailingCheck(31., "short"))
	assert.False(t, s.trailingCheck(30.96, "short"))
	assert.False(t, s.trailingCheck(30.96, "long"))
	assert.False(t, s.trailingCheck(30.95, "short"))
	assert.True(t, s.trailingCheck(30.95, "long"))
}

func Test_TrailingCheckShort(t *testing.T) {
	s := &Strategy{}
	s.lowestPrice = 30.
	s.sellPrice = 30.
	s.TrailingActivationRatio = []float64{0.002, 0.01}
	s.TrailingCallbackRate = []float64{0.0008, 0.0016}
	assert.False(t, s.trailingCheck(29.96, "long"))
	assert.False(t, s.trailingCheck(29.96, "short"))
	assert.False(t, s.trailingCheck(29.99, "short"))
	assert.False(t, s.trailingCheck(29.99, "long"))
	assert.False(t, s.trailingCheck(29.93, "short"))
	assert.False(t, s.trailingCheck(29.93, "long"))
	assert.True(t, s.trailingCheck(29.96, "short"))
	assert.False(t, s.trailingCheck(29.96, "long"))
}
