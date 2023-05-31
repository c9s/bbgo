package indicator

import (
	"testing"

	"github.com/c9s/bbgo/pkg/types"
	"github.com/stretchr/testify/assert"
)

func Test_VIDYA(t *testing.T) {
	vidya := &VIDYA{IntervalWindow: types.IntervalWindow{Window: 16}}
	vidya.Update(1)
	assert.Equal(t, vidya.Last(0), 1.)
	vidya.Update(2)
	newV := 2./17.*2. + 1.*(1.-2./17.)
	assert.Equal(t, vidya.Last(0), newV)
	vidya.Update(1)
	assert.Equal(t, vidya.Last(0), vidya.Index(1))
}
