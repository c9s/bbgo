package bbgo

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

// A fill closes a covered position by moving net and pending together. A reader in
// between must not see a half-applied close: the position would look uncovered and
// the hedge worker would submit the same hedge again.
func TestPositionExposure_CloseIsAtomicForReaders(t *testing.T) {
	for i := 0; i < 2000; i++ {
		pe := NewPositionExposure("BTCUSDT")
		pe.Open(testhelper.Number(1))
		pe.Cover(testhelper.Number(1))

		done := make(chan struct{})
		go func() {
			pe.Close(testhelper.Number(-1))
			close(done)
		}()

		for closed := false; !closed; {
			select {
			case <-done:
				closed = true
			default:
			}

			if uncovered := pe.GetUncovered(); !uncovered.IsZero() {
				t.Fatalf("round %d: a covered position being closed looked uncovered: %s", i, uncovered)
			}
		}
	}
}
