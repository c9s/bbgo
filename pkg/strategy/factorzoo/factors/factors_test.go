package factorzoo

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"

	indicatorv2 "github.com/c9s/bbgo/pkg/indicator/v2"
)

// TestFactors_LegacyGolden compares the factors against the values produced by the legacy
// (pkg/indicator KLineWindowUpdater based) implementation, captured before the migration:
// 150 klines of history in the market data store, factors bound to the store, then 150 live klines.
// The legacy factors processed the whole store window on their first update, so all values are compared.
func TestFactors_LegacyGolden(t *testing.T) {
	kLines := syntheticKLines(300)

	// the shared kline stream already has history, like session.Indicators(symbol).KLines(interval)
	source := &indicatorv2.KLineStream{}
	source.BackFill(kLines[:150])

	pvd := NewPVD(source, 60)
	pmr := NewPMR(source, 60)
	drift := indicatorv2.Drift(indicatorv2.ClosePrices(source), 7)
	mom := NewMOM(source, 1)
	vmom := NewVMOM(source, 90)
	rr := NewRR(source, 2)

	source.BackFill(kLines[150:])

	assertSummary(t, summary{Len: 241, First: -0.44106920140429395, Last: 0.5728862183782256, Sum: 0.6138033687139597}, summarize(pvd.Slice), "PVD")
	assertSummary(t, summary{Len: 182, First: 1.1099150231440487, Last: 1.0724230876405974, Sum: 183.78176429938088}, summarize(pmr.Slice), "PMR")
	assertSummary(t, summary{Len: 293, First: -0.00043452263542022074, Last: 0.0018710247072749593, Sum: -0.18674401870237653}, summarize(drift.Slice), "Drift")
	assertSummary(t, summary{Len: 211, First: 0.8432569499478176, Last: 1.2500433252110859, Sum: 210.66727180857865}, summarize(vmom.Slice), "VMOM")

	// the first MOM and RR values divide by an empty queue slot (+Inf), like the legacy implementation
	assert.True(t, math.IsInf(mom.Slice[0], 1))
	assert.True(t, math.IsInf(rr.Slice[0], 1))
	assertSummary(t, summary{Len: 299, First: -0.0040537056186492615, Last: 0.029077038626255325, Sum: -0.01720927042127829}, summarize(mom.Slice[1:]), "MOM")
	assertSummary(t, summary{Len: 299, First: 0.015003744837436495, Last: 0.022570315239835326, Sum: -0.10142195847409785}, summarize(rr.Slice[1:]), "RR")
}

// TestFactors_WarmUp checks that no value is emitted until the window is filled.
func TestFactors_WarmUp(t *testing.T) {
	kLines := syntheticKLines(200)
	source := &indicatorv2.KLineStream{}
	pvd := NewPVD(source, 60)
	pmr := NewPMR(source, 60)
	vmom := NewVMOM(source, 90)

	source.BackFill(kLines[:59])
	assert.Equal(t, 0, pvd.Length())

	source.BackFill(kLines[59:118])
	assert.Equal(t, 59, pvd.Length())
	assert.Equal(t, 0, pmr.Length()) // PMR needs 60 SMA(60) values: 119 klines

	source.BackFill(kLines[118:])
	assert.Equal(t, 200-119+1, pmr.Length())
	assert.Equal(t, 200-90+1, vmom.Length())
}
