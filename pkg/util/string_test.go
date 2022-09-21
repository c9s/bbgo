package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMaskKey(t *testing.T) {
	type args struct {
		key string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "key length more than 5",
			args: args{key: "abcdefghijklmnopqr"},
			want: "abcde********nopqr",
		},
		{
			name: "key length less than 10",
			args: args{key: "12345678"},
			want: "12****78",
		},
		{
			name: "even",
			args: args{key: "1234567"},
			want: "12***67",
		},
		{
			name: "empty",
			args: args{key: ""},
			want: "{empty}",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MaskKey(tt.args.key); got != tt.want {
				t.Errorf("MaskKey() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStringSplitByLength(t *testing.T) {
	result := StringSplitByLength("1234567890", 3)
	assert.Equal(t, result, []string{"123", "456", "789", "0"})
	result = StringSplitByLength("123許456", 4)
	assert.Equal(t, result, []string{"123", "許4", "56"})
}
