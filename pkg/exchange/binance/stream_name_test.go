package binance

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStreamName(t *testing.T) {
	spot := New("", "")
	assert.Equal(t, "spot", streamName(spot))

	margin := New("", "")
	margin.UseMargin()
	assert.Equal(t, "margin", streamName(margin))

	isolated := New("", "")
	isolated.UseIsolatedMargin("BTCUSDT")
	assert.Equal(t, "isolated_margin", streamName(isolated))

	futures := New("", "")
	futures.UseFutures()
	assert.Equal(t, "futures", streamName(futures))
}
