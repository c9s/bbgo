package types

import (
	"testing"
	"time"

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

func TestBalanceMap_Assets(t *testing.T) {
	type args struct {
		prices    PriceMap
		priceTime time.Time
	}
	tests := []struct {
		name string
		m    BalanceMap
		args args
		want AssetMap
	}{
		{
			m: BalanceMap{
				"USDT": Balance{Currency: "USDT", Available: number(100.0)},
				"BTC":  Balance{Currency: "BTC", Borrowed: number(2.0)},
			},
			args: args{
				prices: PriceMap{
					"BTCUSDT": number(19000.0),
				},
			},
			want: AssetMap{
				"USDT": {
					Currency:   "USDT",
					Total:      number(100),
					NetAsset:   number(100.0),
					Interest:   number(0),
					InUSD:      number(100.0),
					InBTC:      number(100.0 / 19000.0),
					Time:       time.Time{},
					Locked:     number(0),
					Available:  number(100.0),
					Borrowed:   number(0),
					PriceInUSD: number(1.0),
				},
				"BTC": {
					Currency:   "BTC",
					Total:      number(0),
					NetAsset:   number(-2),
					Interest:   number(0),
					InUSD:      number(-2 * 19000.0),
					InBTC:      number(-2),
					Time:       time.Time{},
					Locked:     number(0),
					Available:  number(0),
					Borrowed:   number(2),
					PriceInUSD: number(19000.0),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, tt.m.Assets(tt.args.prices, tt.args.priceTime), "Assets(%v, %v)", tt.args.prices, tt.args.priceTime)
		})
	}
}
