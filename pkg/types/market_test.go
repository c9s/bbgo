package types

import (
	"encoding/json"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

var s func(string) fixedpoint.Value = fixedpoint.MustNewFromString

func TestFormatQuantity(t *testing.T) {
	quantity := formatQuantity(
		s("0.12511"),
		s("0.01"))
	assert.Equal(t, "0.12", quantity)

	quantity = formatQuantity(
		s("0.12511"),
		s("0.001"))
	assert.Equal(t, "0.125", quantity)
}

func TestFormatPrice(t *testing.T) {
	price := formatPrice(
		s("26.288256"),
		s("0.0001"))
	assert.Equal(t, "26.2882", price)

	price = formatPrice(s("26.288656"), s("0.001"))
	assert.Equal(t, "26.288", price)
}

func TestDurationParse(t *testing.T) {
	type A struct {
		Duration Duration `json:"duration"`
	}

	type testcase struct {
		name     string
		input    string
		expected Duration
	}

	var tests = []testcase{
		{
			name:     "int to second",
			input:    `{ "duration": 1 }`,
			expected: Duration(time.Second),
		},
		{
			name:     "float64 to second",
			input:    `{ "duration": 1.1 }`,
			expected: Duration(time.Second + 100*time.Millisecond),
		},
		{
			name:     "2m",
			input:    `{ "duration": "2m" }`,
			expected: Duration(2 * time.Minute),
		},
		{
			name:     "2m3s",
			input:    `{ "duration": "2m3s" }`,
			expected: Duration(2*time.Minute + 3*time.Second),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var a A
			err := json.Unmarshal([]byte(test.input), &a)
			assert.NoError(t, err)
			assert.Equal(t, test.expected, a.Duration)
		})
	}
}

func Test_formatPrice(t *testing.T) {
	type args struct {
		price    fixedpoint.Value
		tickSize fixedpoint.Value
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "no fraction",
			args: args{
				price:    fixedpoint.NewFromFloat(10.0),
				tickSize: fixedpoint.NewFromFloat(0.001),
			},
			want: "10.000",
		},
		{
			name: "fraction truncate",
			args: args{
				price:    fixedpoint.NewFromFloat(2.334),
				tickSize: fixedpoint.NewFromFloat(0.01),
			},
			want: "2.33",
		},
		{
			name: "fraction",
			args: args{
				price:    fixedpoint.NewFromFloat(2.334),
				tickSize: fixedpoint.NewFromFloat(0.0001),
			},
			want: "2.3340",
		},
		{
			name: "more fraction",
			args: args{
				price:    fixedpoint.MustNewFromString("2.1234567898888"),
				tickSize: fixedpoint.NewFromFloat(0.0001),
			},
			want: "2.1234",
		},
	}

	binanceFormatRE := regexp.MustCompile("^([0-9]{1,20})(.[0-9]{1,20})?$")
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatPrice(tt.args.price, tt.args.tickSize)
			if got != tt.want {
				t.Errorf("formatPrice() = %v, want %v", got, tt.want)
			}

			assert.Regexp(t, binanceFormatRE, got)
		})
	}
}

func Test_formatQuantity(t *testing.T) {
	type args struct {
		quantity fixedpoint.Value
		tickSize fixedpoint.Value
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "no fraction",
			args: args{
				quantity: fixedpoint.NewFromFloat(10.0),
				tickSize: fixedpoint.NewFromFloat(0.001),
			},
			want: "10.000",
		},
		{
			name: "fraction truncate",
			args: args{
				quantity: fixedpoint.NewFromFloat(2.334),
				tickSize: fixedpoint.NewFromFloat(0.01),
			},
			want: "2.33",
		},
		{
			name: "fraction",
			args: args{
				quantity: fixedpoint.NewFromFloat(2.334),
				tickSize: fixedpoint.NewFromFloat(0.0001),
			},
			want: "2.3340",
		},
		{
			name: "more fraction",
			args: args{
				quantity: fixedpoint.MustNewFromString("2.1234567898888"),
				tickSize: fixedpoint.NewFromFloat(0.0001),
			},
			want: "2.1234",
		},
	}

	binanceFormatRE := regexp.MustCompile("^([0-9]{1,20})(.[0-9]{1,20})?$")
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatQuantity(tt.args.quantity, tt.args.tickSize)
			if got != tt.want {
				t.Errorf("formatQuantity() = %v, want %v", got, tt.want)
			}

			assert.Regexp(t, binanceFormatRE, got)
		})
	}
}
