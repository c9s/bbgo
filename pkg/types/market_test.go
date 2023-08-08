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

func TestMarket_GreaterThanMinimalOrderQuantity(t *testing.T) {
	market := Market{
		Symbol:          "BTCUSDT",
		LocalSymbol:     "BTCUSDT",
		PricePrecision:  8,
		VolumePrecision: 8,
		QuoteCurrency:   "USDT",
		BaseCurrency:    "BTC",
		MinNotional:     number(10.0),
		MinAmount:       number(10.0),
		MinQuantity:     number(0.0001),
		StepSize:        number(0.00001),
		TickSize:        number(0.01),
	}

	_, ok := market.GreaterThanMinimalOrderQuantity(SideTypeSell, number(20000.0), number(0.00051))
	assert.True(t, ok)

	_, ok = market.GreaterThanMinimalOrderQuantity(SideTypeBuy, number(20000.0), number(10.0))
	assert.True(t, ok)

	_, ok = market.GreaterThanMinimalOrderQuantity(SideTypeBuy, number(20000.0), number(0.99999))
	assert.False(t, ok)
}

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
	price := FormatPrice(
		s("26.288256"),
		s("0.0001"))
	assert.Equal(t, "26.2882", price)

	price = FormatPrice(s("26.288656"), s("0.001"))
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

func Test_FormatPrice(t *testing.T) {
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
			got := FormatPrice(tt.args.price, tt.args.tickSize)
			if got != tt.want {
				t.Errorf("FormatPrice() = %v, want %v", got, tt.want)
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

func TestMarket_TruncateQuantity(t *testing.T) {
	market := Market{
		StepSize: fixedpoint.NewFromFloat(0.0001),
	}

	testCases := []struct {
		input  string
		expect string
	}{
		{"0.00573961", "0.0057"},
		{"0.00579961", "0.0057"},
		{"0.0057", "0.0057"},
	}

	for _, testCase := range testCases {
		q := fixedpoint.MustNewFromString(testCase.input)
		q2 := market.TruncateQuantity(q)
		assert.Equalf(t, testCase.expect, q2.String(), "input: %s stepSize: %s", testCase.input, market.StepSize.String())
	}

}

func TestMarket_AdjustQuantityByMinNotional(t *testing.T) {
	market := Market{
		Symbol:          "ETHUSDT",
		StepSize:        fixedpoint.NewFromFloat(0.0001),
		MinQuantity:     fixedpoint.NewFromFloat(0.00045),
		MinNotional:     fixedpoint.NewFromFloat(10.0),
		VolumePrecision: 8,
		PricePrecision:  2,
	}

	// Quantity:0.00573961 Price:1750.99
	testCases := []struct {
		input  string
		price  fixedpoint.Value
		expect fixedpoint.Value
	}{
		{"0.00573961", number(1750.99), number("0.005739")},
		{"0.0019", number(1757.38), number("0.0057")},
	}

	for _, testCase := range testCases {
		q := fixedpoint.MustNewFromString(testCase.input)
		q2 := market.AdjustQuantityByMinNotional(q, testCase.price)
		assert.InDelta(t, testCase.expect.Float64(), q2.Float64(), 0.000001, "input: %s stepSize: %s", testCase.input, market.StepSize.String())
		assert.False(t, market.IsDustQuantity(q2, testCase.price))
	}
}
