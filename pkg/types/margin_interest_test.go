package types

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

func TestMarginInterest_BorrowedAndNominal(t *testing.T) {
	paid := MarginInterest{
		Asset:        "USDT",
		Principle:    fixedpoint.MustNewFromString("1000"),
		Interest:     fixedpoint.MustNewFromString("0.00417"),
		InterestRate: fixedpoint.MustNewFromString("0.0001"),
	}
	assert.Equal(t, "USDT", paid.BorrowedAsset())
	assert.False(t, paid.IsConverted())
	assert.Equal(t, "0.00417", paid.NominalInterest().String())

	// conversion is decided by the type, not by RawAsset != Asset
	sameAsset := paid
	sameAsset.RawAsset = "USDT"
	sameAsset.InterestType = "PERIODIC"
	assert.False(t, sameAsset.IsConverted())
	assert.Equal(t, "0.00417", sameAsset.NominalInterest().String())

	// paid in BNB: nominal is the hourly charge of the daily rate, in the borrowed asset
	converted := MarginInterest{
		Asset:        "BNB",
		RawAsset:     "BTC",
		InterestType: MarginInterestTypePeriodicConverted,
		Principle:    fixedpoint.MustNewFromString("0.5"),
		Interest:     fixedpoint.MustNewFromString("0.000003"),
		InterestRate: fixedpoint.MustNewFromString("0.00024"),
	}
	assert.Equal(t, "BTC", converted.BorrowedAsset())
	assert.True(t, converted.IsConverted())
	assert.InDelta(t, 0.000005, converted.NominalInterest().Float64(), 1e-12)
}
