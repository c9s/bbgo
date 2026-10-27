package indicatorv2

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/types"
)

func Test_UtBotAlert(t *testing.T) {
	kLines := randomWalkKLines(500, 4)

	stream := &KLineStream{}
	ut := UtBotAlert(stream, 10, 1.0)
	stream.BackFill(kLines)

	legacy := indicator.NewUtBotAlert(types.IntervalWindow{Window: 10}, 1.0)
	legacy.LoadK(kLines)

	require.Equal(t, legacy.Length(), ut.Length())
	require.Equal(t, len(kLines), ut.TrailingStop.Length())

	var ups, downs int
	for i := 0; i < ut.Length(); i++ {
		assert.Equal(t, legacy.Index(i), ut.Direction(i), "signal at index %d", i)
		switch ut.Direction(i) {
		case types.DirectionUp:
			ups++
		case types.DirectionDown:
			downs++
		}
	}

	// make sure the data set actually exercises both signals
	assert.Greater(t, ups, 0)
	assert.Greater(t, downs, 0)
	assert.Equal(t, legacy.GetSignal(), ut.GetSignal())
}
