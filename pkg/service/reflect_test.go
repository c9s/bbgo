package service

import (
	"reflect"
	"testing"

	"github.com/c9s/bbgo/pkg/types"
)

func Test_tableNameOf(t *testing.T) {
	type args struct {
		record interface{}
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "MarginInterest",
			args: args{record: &types.MarginInterest{}},
			want: "margin_interests",
		},
		{
			name: "MarginLoan",
			args: args{record: &types.MarginLoan{}},
			want: "margin_loans",
		},
		{
			name: "MarginRepay",
			args: args{record: &types.MarginRepay{}},
			want: "margin_repays",
		},
		{
			name: "MarginLiquidation",
			args: args{record: &types.MarginLiquidation{}},
			want: "margin_liquidations",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tableNameOf(tt.args.record); got != tt.want {
				t.Errorf("tableNameOf() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_fieldsNamesOf(t *testing.T) {
	type args struct {
		record interface{}
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "MarginInterest",
			args: args{record: &types.MarginInterest{}},
			want: []string{"exchange", "asset", "principle", "interest", "interest_rate", "isolated_symbol", "time"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := fieldsNamesOf(tt.args.record); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("fieldsNamesOf() = %v, want %v", got, tt.want)
			}
		})
	}
}
