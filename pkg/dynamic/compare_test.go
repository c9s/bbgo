package dynamic

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	. "github.com/c9s/bbgo/pkg/testing/testhelper"
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

func Test_Compare(t *testing.T) {
	tests := []struct {
		name    string
		a, b    interface{}
		want    []Diff
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name:    "order",
			wantErr: assert.NoError,
			a: &types.Order{
				SubmitOrder: types.SubmitOrder{
					Symbol:   "BTCUSDT",
					Quantity: fixedpoint.NewFromFloat(100.0),
				},
				Status:           types.OrderStatusFilled,
				ExecutedQuantity: fixedpoint.NewFromFloat(100.0),
			},
			b: &types.Order{
				SubmitOrder: types.SubmitOrder{
					Symbol:   "BTCUSDT",
					Quantity: fixedpoint.NewFromFloat(100.0),
				},
				ExecutedQuantity: fixedpoint.NewFromFloat(50.0),
				Status:           types.OrderStatusPartiallyFilled,
			},
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
		{
			name:    "kline",
			wantErr: assert.NoError,
			a: types.KLine{
				Open:  Number(60000),
				High:  Number(61000),
				Low:   Number(59500),
				Close: Number(60100),
			},
			b: types.KLine{
				Open:  Number(60000),
				High:  Number(61000),
				Low:   Number(59500),
				Close: Number(60200),
			},
			want: []Diff{
				{
					Field:  "Close",
					Before: "60200",
					After:  "60100",
				},
			},
		},
		{
			name:    "kline ptr",
			wantErr: assert.NoError,
			a: &types.KLine{
				Open:  Number(60000),
				High:  Number(61000),
				Low:   Number(59500),
				Close: Number(60100),
			},
			b: &types.KLine{
				Open:  Number(60000),
				High:  Number(61000),
				Low:   Number(59500),
				Close: Number(60200),
			},
			want: []Diff{
				{
					Field:  "Close",
					Before: "60200",
					After:  "60100",
				},
			},
		},
		{
			name:    "deposit and order",
			wantErr: assert.NoError,
			a: &types.Deposit{
				Address:       "0x6666",
				TransactionID: "0x3333",
				Status:        types.DepositPending,
				Confirmation:  "10/15",
			},
			b: &types.Deposit{
				Address:       "0x6666",
				TransactionID: "0x3333",
				Status:        types.DepositPending,
				Confirmation:  "1/15",
			},
			want: []Diff{
				{
					Field:  "Confirmation",
					Before: "1/15",
					After:  "10/15",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Compare(tt.a, tt.b)
			if !tt.wantErr(t, err, fmt.Sprintf("Compare(%v, %v)", tt.a, tt.b)) {
				return
			}

			assert.Equalf(t, tt.want, got, "Compare(%v, %v)", tt.a, tt.b)
		})
	}
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
