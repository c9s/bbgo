package fmaker

import (
	"testing"

	"github.com/stretchr/testify/assert"

	indicatorv2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

// buildFactors builds the factors with the same windows as Strategy.Run
func buildFactors(source *indicatorv2.KLineStream) map[string]*types.Float64Series {
	return map[string]*types.Float64Series{
		"R":   NewR(source, 1).Float64Series,
		"S0":  NewS0(source, 20).Float64Series,
		"S1":  NewS1(source, 20).Float64Series,
		"S2":  NewS2(source, 20).Float64Series,
		"S3":  NewS3(source, 2).Float64Series,
		"S4":  NewS4(source, 2).Float64Series,
		"S5":  NewS5(source, 10).Float64Series,
		"S6":  NewS6(source, 2).Float64Series,
		"S7":  NewS7(source, 2).Float64Series,
		"A2":  NewA2(source, 2).Float64Series,
		"A3":  NewA3(source, 8).Float64Series,
		"A18": NewA18(source, 5).Float64Series,
		"A34": NewA34(source, 12).Float64Series,
	}
}

// TestFactors_LegacyGolden compares the factors against the values produced by the legacy
// (pkg/indicator KLineWindowUpdater based) implementation, captured before the migration:
// 100 klines of history in the market data store, factors bound to the store, then 200 live klines.
// The legacy factors only processed the live klines, so the last 200 values are compared.
func TestFactors_LegacyGolden(t *testing.T) {
	golden := map[string]summary{
		"R":   {Len: 200, First: -0.012162633489180674, Last: 0.006363106076371805, Sum: 0.05072707654682529},
		"S0":  {Len: 200, First: 0.8764549287832195, Last: 0.9990064074343371, Sum: 192.339826502131},
		"S1":  {Len: 200, First: -0.5694685225724773, Last: -0.6313545206011265, Sum: -16.230037990552884},
		"S2":  {Len: 200, First: 0.9449962853905468, Last: 0.9475368863822086, Sum: 0.7645424791552042},
		"S3":  {Len: 200, First: 0.9859268905372578, Last: 1.0290770386262553, Sum: 199.88256450844565},
		"S4":  {Len: 200, First: 0.009073488481343565, Last: 0.010891089866650381, Sum: 2.0163090849343455},
		"S5":  {Len: 200, First: -1.0169203396842554, Last: -0.9849509177852871, Sum: -203.28249127347402},
		"S6":  {Len: 200, First: 81767.75816525202, Last: 114643.28708718135, Sum: 2.0042092366888344e+07},
		"S7":  {Len: 200, First: -0.012162633489180674, Last: 0.006363106076371805, Sum: 0.05072707654682529},
		"A2":  {Len: 200, First: -0.13291817996660923, Last: -0.24610557994528026, Sum: 0.1079747464293474},
		"A3":  {Len: 200, First: -16.046092640000012, Last: -11.150800320000002, Sum: 170.97771366999947},
		"A18": {Len: 200, First: 1.004529276820688, Last: 1.0541358317337002, Sum: 199.44097053676398},
		"A34": {Len: 200, First: 0.9833374918839884, Last: 0.9903900112546594, Sum: 201.62543031794047},
	}

	kLines := syntheticKLines(300)

	// the shared kline stream already has history, like session.Indicators(symbol).KLines(interval)
	shared := &indicatorv2.KLineStream{}
	shared.BackFill(kLines[:100])

	fs := indicatorv2.WithHistory(shared, buildFactors)

	// a handler registered after the factors are built sees the factors updated with the current kline
	r := fs["R"]
	a3 := fs["A3"]
	shared.OnUpdate(func(k types.KLine) {
		expected, err := calculateR([]types.KLine{k}, types.KLineOpenPriceMapper, types.KLineClosePriceMapper)
		assert.NoError(t, err)
		assert.Equal(t, expected, r.Last(0))

		window := shared.Tail(8)
		expected, err = calculateA3(window, KLineLowPriceMapper, KLineHighPriceMapper, types.KLineClosePriceMapper)
		assert.NoError(t, err)
		assert.Equal(t, expected, a3.Last(0))
	})

	shared.BackFill(kLines[100:])

	for name, expected := range golden {
		values := fs[name].Slice
		assert.Equal(t, expected, summarize(values[len(values)-200:]), name)
	}

	// the history is processed too: one value per kline once the window is filled
	assert.Equal(t, 300, fs["R"].Length())
	assert.Equal(t, 300-20+1, fs["S0"].Length())
	assert.Equal(t, 300-8+1, fs["A3"].Length())
}

// TestFactors_ColdStart checks the warm-up: no value is emitted until `window` klines are available.
func TestFactors_ColdStart(t *testing.T) {
	kLines := syntheticKLines(50)
	source := &indicatorv2.KLineStream{}
	s0 := NewS0(source, 20)
	a3 := NewA3(source, 8)

	source.BackFill(kLines[:19])
	assert.Equal(t, 0, s0.Length())

	source.BackFill(kLines[19:])

	// captured from the legacy implementation fed by an empty market data store
	assert.Equal(t, summary{Len: 31, First: 0.9667557510695416, Last: 0.8593215474458797, Sum: 30.51985001871105}, summarize(s0.Slice))
	assert.Equal(t, summary{Len: 43, First: 12.155941509999977, Last: -16.594099160000013, Sum: 74.78138925999983}, summarize(a3.Slice))
}
