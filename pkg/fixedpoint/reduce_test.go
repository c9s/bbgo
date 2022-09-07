package fixedpoint

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReduce(t *testing.T) {
	type args struct {
		values  []Value
		init    Value
		reducer Reducer
	}
	tests := []struct {
		name string
		args args
		want Value
	}{
		{
			name: "simple",
			args: args{
				values: []Value{NewFromFloat(1), NewFromFloat(2), NewFromFloat(3)},
				init:   NewFromFloat(0.0),
				reducer: func(prev, curr Value) Value {
					return prev.Add(curr)
				},
			},
			want: NewFromFloat(6),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, Reduce(tt.args.values, tt.args.reducer, tt.args.init), "Reduce(%v, %v, %v)", tt.args.values, tt.args.init, tt.args.reducer)
		})
	}
}
