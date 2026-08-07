//go:build !dnum

package xmaker

import (
	"encoding/json"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/dynamic"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	. "github.com/c9s/bbgo/pkg/testing/testhelper"
)

func Test_aggregatePrice(t *testing.T) {
	bids := PriceVolumeSliceFromText(`
	1000.0, 1.0
	1200.0, 1.0
	1400.0, 1.0
`)

	aggregatedPrice1 := aggregatePrice(bids, fixedpoint.NewFromFloat(0.5))
	assert.Equal(t, fixedpoint.NewFromFloat(1000.0), aggregatedPrice1)

	aggregatedPrice2 := aggregatePrice(bids, fixedpoint.NewFromInt(1))
	assert.Equal(t, fixedpoint.NewFromFloat(1000.0), aggregatedPrice2)

	aggregatedPrice3 := aggregatePrice(bids, fixedpoint.NewFromInt(2))
	assert.Equal(t, fixedpoint.NewFromFloat(1100.0), aggregatedPrice3)
}

func Test_calculateSpread(t *testing.T) {
	tests := []struct {
		name string
		a    fixedpoint.Value
		b    fixedpoint.Value
		want fixedpoint.Value
	}{
		{
			name: "positive spread",
			a:    fixedpoint.NewFromFloat(100.0),
			b:    fixedpoint.NewFromFloat(90.0),
			want: fixedpoint.NewFromFloat(0.1), // (100 - 90) / 100 = 0.1
		},
		{
			name: "negative spread",
			a:    fixedpoint.NewFromFloat(100.0),
			b:    fixedpoint.NewFromFloat(110.0),
			want: fixedpoint.NewFromFloat(-0.1), // (100 - 110) / 100 = -0.1
		},
		{
			name: "zero spread",
			a:    fixedpoint.NewFromFloat(100.0),
			b:    fixedpoint.NewFromFloat(100.0),
			want: fixedpoint.Zero,
		},
		{
			name: "zero divisor",
			a:    fixedpoint.Zero,
			b:    fixedpoint.NewFromFloat(100.0),
			want: fixedpoint.Zero,
		},
		{
			name: "both zero",
			a:    fixedpoint.Zero,
			b:    fixedpoint.Zero,
			want: fixedpoint.Zero,
		},
		{
			name: "negative a",
			a:    fixedpoint.NewFromFloat(-100.0),
			b:    fixedpoint.NewFromFloat(-90.0),
			want: fixedpoint.NewFromFloat(0.1), // (-100 - (-90)) / -100 = -10 / -100 = 0.1
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculateSpread(tt.a, tt.b)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_EmbeddedConfigFlatten(t *testing.T) {
	const blob = `{
	  "symbol": "BTCUSDT",
	  "makerSymbol": "BTCUSDT",
	  "makerExchange": "max",
	  "sourceExchange": "binance",
	  "margin": "0.0015",
	  "quantity": "0.01",
	  "numLayers": 3,
	  "circuitBreaker": {}
	}`

	s := &Strategy{}
	if err := json.Unmarshal([]byte(blob), s); err != nil {
		t.Fatal(err)
	}

	// promoted (embedded StrategyConfig) fields
	assert.Equal(t, "0.0015", s.Margin.String())
	assert.Equal(t, "0.01", s.Quantity.String())
	assert.Equal(t, 3, s.NumLayers)
	assert.Equal(t, "max", s.MakerExchange)
	assert.NotNil(t, s.CircuitBreaker)

	// top-level fields
	assert.Equal(t, "BTCUSDT", s.Symbol)
	assert.Equal(t, "BTCUSDT", s.MakerSymbol)

	// config metrics still register for promoted fields
	assert.NoError(t, dynamic.InitializeConfigMetrics(ID, "xmaker:BTCUSDT:temp", s))
}

func newLinearLayerScale(domain, rng [2]float64) *bbgo.LayerScale {
	return &bbgo.LayerScale{
		LayerRule: &bbgo.SlideRule{
			LinearScale: &bbgo.LinearScale{
				Domain: domain,
				Range:  rng,
			},
		},
	}
}

func TestStrategy_getInitialLayerQuantity_QuantityScale(t *testing.T) {
	// layer 1 -> 10, layer 3 -> 30 (linear)
	price := fixedpoint.NewFromFloat(100.0)

	t.Run("quote scale disabled: quantity scale is already in base currency", func(t *testing.T) {
		s := &Strategy{
			logger: logrus.New(),
			StrategyConfig: StrategyConfig{
				QuantityScale: newLinearLayerScale([2]float64{1, 3}, [2]float64{10, 30}),
			},
		}

		q, err := s.getInitialLayerQuantity(0, price)
		assert.NoError(t, err)
		assert.InDelta(t, 10.0, q.Float64(), 1e-8)

		q, err = s.getInitialLayerQuantity(1, price)
		assert.NoError(t, err)
		assert.InDelta(t, 20.0, q.Float64(), 1e-8)
	})

	t.Run("quote scale enabled: quantity scale is in quote currency and converted to base", func(t *testing.T) {
		s := &Strategy{
			logger: logrus.New(),
			StrategyConfig: StrategyConfig{
				QuantityScale:    newLinearLayerScale([2]float64{1, 3}, [2]float64{10, 30}),
				UseQuoteQuantity: true,
			},
		}

		// layer 1 -> quote 10 -> base 10/100 = 0.1
		q, err := s.getInitialLayerQuantity(0, price)
		assert.NoError(t, err)
		assert.InDelta(t, 0.1, q.Float64(), 1e-8)

		// layer 2 -> quote 20 -> base 20/100 = 0.2
		q, err = s.getInitialLayerQuantity(1, price)
		assert.NoError(t, err)
		assert.InDelta(t, 0.2, q.Float64(), 1e-8)
	})

	t.Run("quote scale enabled but price is zero: no conversion is applied", func(t *testing.T) {
		s := &Strategy{
			logger: logrus.New(),
			StrategyConfig: StrategyConfig{
				QuantityScale:    newLinearLayerScale([2]float64{1, 3}, [2]float64{10, 30}),
				UseQuoteQuantity: true,
			},
		}

		q, err := s.getInitialLayerQuantity(0, fixedpoint.Zero)
		assert.NoError(t, err)
		assert.InDelta(t, 10.0, q.Float64(), 1e-8)
	})
}

func TestStrategy_getInitialLayerQuantity_SimpleMultiplier(t *testing.T) {
	price := fixedpoint.NewFromFloat(100.0)

	t.Run("no multiplier, quote scale disabled: quantity is fixed", func(t *testing.T) {
		s := &Strategy{
			logger: logrus.New(),
			StrategyConfig: StrategyConfig{
				Quantity: fixedpoint.NewFromFloat(10.0),
			},
		}

		for i := 0; i < 3; i++ {
			q, err := s.getInitialLayerQuantity(i, price)
			assert.NoError(t, err)
			assert.InDelta(t, 10.0, q.Float64(), 1e-8)
		}
	})

	t.Run("multiplier applied for layers after the first, quote scale disabled", func(t *testing.T) {
		s := &Strategy{
			logger: logrus.New(),
			StrategyConfig: StrategyConfig{
				Quantity:           fixedpoint.NewFromFloat(10.0),
				QuantityMultiplier: fixedpoint.NewFromFloat(1.1),
			},
		}

		q, err := s.getInitialLayerQuantity(0, price)
		assert.NoError(t, err)
		assert.InDelta(t, 10.0, q.Float64(), 1e-8)

		q, err = s.getInitialLayerQuantity(1, price)
		assert.NoError(t, err)
		assert.InDelta(t, 10.0*1.1*1.1, q.Float64(), 1e-8)

		q, err = s.getInitialLayerQuantity(2, price)
		assert.NoError(t, err)
		assert.InDelta(t, 10.0*1.1*1.1*1.1, q.Float64(), 1e-8)
	})

	t.Run("quote scale enabled: fixed quantity is in quote currency and converted to base", func(t *testing.T) {
		s := &Strategy{
			logger: logrus.New(),
			StrategyConfig: StrategyConfig{
				Quantity:         fixedpoint.NewFromFloat(1000.0),
				UseQuoteQuantity: true,
			},
		}

		// 1000 quote / 100 price = 10 base
		q, err := s.getInitialLayerQuantity(0, price)
		assert.NoError(t, err)
		assert.InDelta(t, 10.0, q.Float64(), 1e-8)
	})

	t.Run("quote scale and multiplier both enabled", func(t *testing.T) {
		s := &Strategy{
			logger: logrus.New(),
			StrategyConfig: StrategyConfig{
				Quantity:           fixedpoint.NewFromFloat(1000.0),
				QuantityMultiplier: fixedpoint.NewFromFloat(1.1),
				UseQuoteQuantity:   true,
			},
		}

		// layer 0: 1000/100 = 10, multiplier not applied
		q, err := s.getInitialLayerQuantity(0, price)
		assert.NoError(t, err)
		assert.InDelta(t, 10.0, q.Float64(), 1e-8)

		// layer 1: (1000/100) * 1.1^2 = 12.1
		q, err = s.getInitialLayerQuantity(1, price)
		assert.NoError(t, err)
		assert.InDelta(t, 10.0*1.1*1.1, q.Float64(), 1e-8)
	})

	t.Run("quote scale enabled but price is zero: no conversion is applied", func(t *testing.T) {
		s := &Strategy{
			logger: logrus.New(),
			StrategyConfig: StrategyConfig{
				Quantity:         fixedpoint.NewFromFloat(1000.0),
				UseQuoteQuantity: true,
			},
		}

		q, err := s.getInitialLayerQuantity(0, fixedpoint.Zero)
		assert.NoError(t, err)
		assert.InDelta(t, 1000.0, q.Float64(), 1e-8)
	})
}
