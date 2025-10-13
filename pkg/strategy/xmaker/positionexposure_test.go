package xmaker

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/testing/testhelper"
)

func TestPositionExposure(t *testing.T) {
	pe := NewPositionExposure("BTCUSDT")

	// initial value
	assert.Equal(t, testhelper.Number(0), pe.net.Get())
	assert.Equal(t, testhelper.Number(0), pe.pending.Get())
	assert.Equal(t, testhelper.Number(0), pe.GetUncovered())

	// open position (maker orders are filled)
	pe.Open(testhelper.Number(2))
	assert.Equal(t, testhelper.Number(2), pe.net.Get())
	assert.Equal(t, testhelper.Number(0), pe.pending.Get())
	assert.Equal(t, testhelper.Number(2), pe.GetUncovered())

	// cover 1 (hedge order is placed, and pending is updated)
	pe.Cover(testhelper.Number(1))
	assert.Equal(t, testhelper.Number(2), pe.net.Get())
	assert.Equal(t, testhelper.Number(1), pe.pending.Get())
	assert.Equal(t, testhelper.Number(1), pe.GetUncovered())

	// close 1 (hedge order is filled, pending is updated)
	pe.Close(testhelper.Number(-1))
	assert.Equal(t, testhelper.Number(1), pe.net.Get())
	assert.Equal(t, testhelper.Number(0), pe.pending.Get())
	assert.Equal(t, testhelper.Number(1), pe.GetUncovered())

	// open 1 (another maker order is filled, pending is updated)
	pe.Open(testhelper.Number(-1))
	assert.Equal(t, testhelper.Number(0), pe.net.Get())
	assert.Equal(t, testhelper.Number(0), pe.pending.Get())
	assert.Equal(t, testhelper.Number(0), pe.GetUncovered())
}
