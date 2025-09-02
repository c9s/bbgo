package bfxapi

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOrderStatusUnmarshalJSON(t *testing.T) {
	testCases := []struct {
		input    string
		expected OrderStatus
	}{
		{"ACTIVE", OrderStatusActive},
		{"EXECUTED @ 107.6(-0.2)", OrderStatusExecuted},
		{"FILLED @ 100.0(1.0)", OrderStatusExecuted}, // FILLED is an alias for EXECUTED
		{"PARTIALLY FILLED @ 105.0(0.5)", OrderStatusPartiallyFilled},
		{"PARTIALLY EXECUTED @ 105.0(0.5)", OrderStatusPartiallyFilled},
		{"CANCELED", OrderStatusCanceled},
		{"CANCELLED", OrderStatusCanceled},
		{"POSTONLY CANCELED", OrderStatusCanceled},
		{"IOC CANCELED", OrderStatusCanceled},
		{"CANCELED was: PARTIALLY FILLED @ 105.0(0.5)", OrderStatusCanceled},
		{"REJECTED", OrderStatusRejected},
		{"EXPIRED", OrderStatusExpired},
		{"INSUFFICIENT MARGIN was: PARTIALLY FILLED @ 105.0(0.5)", OrderStatusInsufficientBal},
		{"RSN_DUST (amount is less than 0.00000001)", OrderStatusRejected},
		{"RSN_PAUSE (trading is paused due to rebase events on AMPL or funding settlement on derivatives)", OrderStatusRejected},
	}

	for _, tc := range testCases {
		var status OrderStatus
		data, err := json.Marshal(tc.input)
		assert.NoError(t, err)
		assert.NoError(t, status.UnmarshalJSON(data), "input: %s", tc.input)
		assert.Equal(t, tc.expected, status, "input: %s", tc.input)
	}
}
