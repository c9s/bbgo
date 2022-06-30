package dynamic

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type TestStrategy struct {
	Symbol            string           `json:"symbol"`
	Interval          string           `json:"interval"`
	BaseQuantity      fixedpoint.Value `json:"baseQuantity"`
	MaxAssetQuantity  fixedpoint.Value `json:"maxAssetQuantity"`
	MinDropPercentage fixedpoint.Value `json:"minDropPercentage"`
}

func Test_reflectMergeStructFields(t *testing.T) {
	t.Run("zero value", func(t *testing.T) {
		a := &TestStrategy{Symbol: "BTCUSDT"}
		b := &struct{ Symbol string }{Symbol: ""}
		InheritStructValues(b, a)
		assert.Equal(t, "BTCUSDT", b.Symbol)
	})

	t.Run("non-zero value", func(t *testing.T) {
		a := &TestStrategy{Symbol: "BTCUSDT"}
		b := &struct{ Symbol string }{Symbol: "ETHUSDT"}
		InheritStructValues(b, a)
		assert.Equal(t, "ETHUSDT", b.Symbol, "should be the original value")
	})

	t.Run("zero embedded struct", func(t *testing.T) {
		iw := types.IntervalWindow{Interval: types.Interval1h, Window: 30}
		a := &struct {
			types.IntervalWindow
			Symbol string
		}{
			IntervalWindow: iw,
			Symbol:         "BTCUSDT",
		}
		b := &struct {
			Symbol string
			types.IntervalWindow
		}{}
		InheritStructValues(b, a)
		assert.Equal(t, iw, b.IntervalWindow)
		assert.Equal(t, "BTCUSDT", b.Symbol)
	})

	t.Run("non-zero embedded struct", func(t *testing.T) {
		iw := types.IntervalWindow{Interval: types.Interval1h, Window: 30}
		a := &struct {
			types.IntervalWindow
		}{
			IntervalWindow: iw,
		}
		b := &struct {
			types.IntervalWindow
		}{
			IntervalWindow: types.IntervalWindow{Interval: types.Interval5m, Window: 9},
		}
		InheritStructValues(b, a)
		assert.Equal(t, types.IntervalWindow{Interval: types.Interval5m, Window: 9}, b.IntervalWindow)
	})

	t.Run("skip different type but the same name", func(t *testing.T) {
		a := &struct {
			A float64
		}{
			A: 1.99,
		}
		b := &struct {
			A string
		}{}
		InheritStructValues(b, a)
		assert.Equal(t, "", b.A)
		assert.Equal(t, 1.99, a.A)
	})
}
