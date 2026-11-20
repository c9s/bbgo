package btc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBlockSubsidy(t *testing.T) {
	assert.Equal(t, 50*Amount(SatoshiPerBitcoin), BlockSubsidy(0))
	assert.Equal(t, 50*Amount(SatoshiPerBitcoin), BlockSubsidy(209_999))
	assert.Equal(t, 25*Amount(SatoshiPerBitcoin), BlockSubsidy(210_000))
	assert.Equal(t, Amount(312_500_000), BlockSubsidy(840_000)) // the fourth halving, April 2024
	assert.Equal(t, Amount(312_500_000), BlockSubsidy(975_765))
	assert.Equal(t, Amount(156_250_000), BlockSubsidy(1_050_000))
	assert.Equal(t, Amount(1), BlockSubsidy(32*HalvingInterval))
	assert.Equal(t, Amount(0), BlockSubsidy(33*HalvingInterval))
	assert.Equal(t, Amount(0), BlockSubsidy(64*HalvingInterval))
	assert.Equal(t, Amount(0), BlockSubsidy(-1))
}
