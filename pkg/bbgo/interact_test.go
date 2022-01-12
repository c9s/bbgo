package bbgo

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_parseFuncArgsAndCall_NoErrorFunction(t *testing.T) {
	noErrorFunc := func(a string, b float64, c bool) error {
		assert.Equal(t, "BTCUSDT", a)
		assert.Equal(t, 0.123, b)
		assert.Equal(t, true, c)
		return nil
	}

	err := parseFuncArgsAndCall(noErrorFunc, []string{"BTCUSDT", "0.123", "true"})
	assert.NoError(t, err)
}

func Test_parseFuncArgsAndCall_ErrorFunction(t *testing.T) {
	errorFunc := func(a string, b float64) error {
		return errors.New("error")
	}

	err := parseFuncArgsAndCall(errorFunc, []string{"BTCUSDT", "0.123"})
	assert.Error(t, err)

}

func Test_parseCommand(t *testing.T) {
	args := parseCommand(`closePosition "BTC USDT" 3.1415926 market`)
	t.Logf("args: %+v", args)
	for i, a := range args {
		t.Logf("args(%d): %#v", i, a)
	}

	assert.Equal(t, 4, len(args))
	assert.Equal(t, "closePosition", args[0])
	assert.Equal(t, "BTC USDT", args[1])
	assert.Equal(t, "3.1415926", args[2])
	assert.Equal(t, "market", args[3])
}
