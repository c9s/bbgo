//go:build dnum

package fixedpoint

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestDelta(t *testing.T) {
	f1 := MustNewFromString("0.0009763593380614657")
	f2 := NewFromInt(42300)
	assert.InDelta(t, f1.Mul(f2).Float64(), 41.3, 1e-14)
}
