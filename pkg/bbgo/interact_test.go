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

}

