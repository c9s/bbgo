package indicator

import (
	"testing"
	"time"

	"github.com/c9s/bbgo/pkg/types"
	"github.com/stretchr/testify/assert"
)

func Test_DelVolumeProfile(t *testing.T) {

	vp := VolumeProfile{IntervalWindow: types.IntervalWindow{Window: 1, Interval: types.Interval1s}, Delta: 1.0}
	vp.Update(1., 100., types.Time(time.Now()))
	r, v := vp.PointOfControlAboveEqual(1.)
	assert.Equal(t, r, 1.)
	assert.Equal(t, v, 100.)
	vp.Update(2., 100., types.Time(time.Now().Add(time.Second*10)))
	r, v = vp.PointOfControlAboveEqual(1.)
	assert.Equal(t, r, 2.)
	assert.Equal(t, v, 100.)
	r, v = vp.PointOfControlBelowEqual(1.)
	assert.Equal(t, r, 0.)
	assert.Equal(t, v, 0.)
	r, v = vp.PointOfControlBelowEqual(2.)
	assert.Equal(t, r, 2.)
	assert.Equal(t, v, 100.)

}
