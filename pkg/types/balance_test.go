package types

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

func TestBalanceMap_Add(t *testing.T) {
	var bm = BalanceMap{}
	var bm2 = bm.Add(BalanceMap{
		"BTC": Balance{
			Currency:  "BTC",
			Available: fixedpoint.MustNewFromString("10.0"),
			Locked:    fixedpoint.MustNewFromString("0"),
			NetAsset:  fixedpoint.MustNewFromString("10.0"),
		},
	})
	assert.Len(t, bm2, 1)

	var bm3 = bm2.Add(BalanceMap{
		"BTC": Balance{
			Currency:  "BTC",
			Available: fixedpoint.MustNewFromString("1.0"),
			Locked:    fixedpoint.MustNewFromString("0"),
			NetAsset:  fixedpoint.MustNewFromString("1.0"),
		},
		"LTC": Balance{
			Currency:  "LTC",
			Available: fixedpoint.MustNewFromString("20.0"),
			Locked:    fixedpoint.MustNewFromString("0"),
			NetAsset:  fixedpoint.MustNewFromString("20.0"),
		},
	})
	assert.Len(t, bm3, 2)
	assert.Equal(t, fixedpoint.MustNewFromString("11.0"), bm3["BTC"].Available)
}
