package dynamic

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func Test_convertToStr(t *testing.T) {
	t.Run("str-str", func(t *testing.T) {
		out := convertToStr(reflect.ValueOf("a"))
		assert.Equal(t, "a", out)
	})

	t.Run("bool-str", func(t *testing.T) {
		out := convertToStr(reflect.ValueOf(false))
		assert.Equal(t, "false", out)

		out = convertToStr(reflect.ValueOf(true))
		assert.Equal(t, "true", out)
	})

	t.Run("float-str", func(t *testing.T) {
		out := convertToStr(reflect.ValueOf(0.444))
		assert.Equal(t, "0.444", out)
	})

	t.Run("int-str", func(t *testing.T) {
		a := int(123)
		out := convertToStr(reflect.ValueOf(a))
		assert.Equal(t, "123", out)
	})

	t.Run("uint-str", func(t *testing.T) {
		a := uint(123)
		out := convertToStr(reflect.ValueOf(a))
		assert.Equal(t, "123", out)
	})

	t.Run("int-ptr-str", func(t *testing.T) {
		a := int(123)
		out := convertToStr(reflect.ValueOf(&a))
		assert.Equal(t, "123", out)
	})

	t.Run("fixedpoint-str", func(t *testing.T) {
		a := fixedpoint.NewFromInt(100)
		out := convertToStr(reflect.ValueOf(a))
		assert.Equal(t, "100", out)
	})
}

func Test_compareStruct(t *testing.T) {
	tests := []struct {
		name    string
		a, b    reflect.Value
		want    []Diff
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name:    "order ptrs",
			wantErr: assert.NoError,
			a: reflect.ValueOf(&types.Order{
				SubmitOrder: types.SubmitOrder{
					Symbol:   "BTCUSDT",
					Quantity: fixedpoint.NewFromFloat(100.0),
				},
				ExecutedQuantity: fixedpoint.NewFromFloat(50.0),
			}),
			b: reflect.ValueOf(&types.Order{
				SubmitOrder: types.SubmitOrder{
					Symbol:   "BTCUSDT",
					Quantity: fixedpoint.NewFromFloat(100.0),
				},
				ExecutedQuantity: fixedpoint.NewFromFloat(20.0),
			}),
			want: []Diff{
				{
					Field:  "ExecutedQuantity",
					Before: "20",
					After:  "50",
				},
			},
		},
		{
			name:    "order ptr and value",
			wantErr: assert.NoError,
			a: reflect.ValueOf(types.Order{
				SubmitOrder: types.SubmitOrder{
					Symbol:   "BTCUSDT",
					Quantity: fixedpoint.NewFromFloat(100.0),
				},
				Status:           types.OrderStatusFilled,
				ExecutedQuantity: fixedpoint.NewFromFloat(100.0),
			}),
			b: reflect.ValueOf(&types.Order{
				SubmitOrder: types.SubmitOrder{
					Symbol:   "BTCUSDT",
					Quantity: fixedpoint.NewFromFloat(100.0),
				},
				ExecutedQuantity: fixedpoint.NewFromFloat(50.0),
				Status:           types.OrderStatusPartiallyFilled,
			}),
			want: []Diff{
				{
					Field:  "Status",
					Before: "PARTIALLY_FILLED",
					After:  "FILLED",
				},
				{
					Field:  "ExecutedQuantity",
					Before: "50",
					After:  "100",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := compareStruct(tt.a, tt.b)
			if !tt.wantErr(t, err, fmt.Sprintf("compareStruct(%v, %v)", tt.a, tt.b)) {
				return
			}
			assert.Equalf(t, tt.want, got, "compareStruct(%v, %v)", tt.a, tt.b)
		})
	}
}
