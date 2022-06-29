package bbgo

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/types"
)

func Test_reflectMergeStructFields(t *testing.T) {
	t.Run("zero value", func(t *testing.T) {
		a := &TestStrategy{Symbol: "BTCUSDT"}
		b := &CumulatedVolumeTakeProfit{Symbol: ""}
		reflectMergeStructFields(b, a)
		assert.Equal(t, "BTCUSDT", b.Symbol)
	})

	t.Run("non-zero value", func(t *testing.T) {
		a := &TestStrategy{Symbol: "BTCUSDT"}
		b := &CumulatedVolumeTakeProfit{Symbol: "ETHUSDT"}
		reflectMergeStructFields(b, a)
		assert.Equal(t, "ETHUSDT", b.Symbol, "should be the original value")
	})

	t.Run("zero embedded struct", func(t *testing.T) {
		iw := types.IntervalWindow{Interval: types.Interval1h, Window: 30}
		a := &struct {
			types.IntervalWindow
		}{
			IntervalWindow: iw,
		}
		b := &CumulatedVolumeTakeProfit{}
		reflectMergeStructFields(b, a)
		assert.Equal(t, iw, b.IntervalWindow)
	})

	t.Run("non-zero embedded struct", func(t *testing.T) {
		iw := types.IntervalWindow{Interval: types.Interval1h, Window: 30}
		a := &struct {
			types.IntervalWindow
		}{
			IntervalWindow: iw,
		}
		b := &CumulatedVolumeTakeProfit{
			IntervalWindow: types.IntervalWindow{Interval: types.Interval5m, Window: 9},
		}
		reflectMergeStructFields(b, a)
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
		reflectMergeStructFields(b, a)
		assert.Equal(t, "", b.A)
		assert.Equal(t, 1.99, a.A)
	})

}
