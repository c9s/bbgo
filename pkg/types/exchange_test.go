package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_exchangeName(t *testing.T) {
	assert.Equal(t, ExchangeMax.String(), "max")
	name, err := ValidExchangeName("binance")
	assert.Equal(t, name, ExchangeName("binance"))
	assert.NoError(t, err)
	_, err = ValidExchangeName("dummy")
	assert.Error(t, err)
	assert.True(t, ExchangeMax.IsValid())
}
