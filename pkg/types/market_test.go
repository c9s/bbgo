package types

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/stretchr/testify/assert"
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
