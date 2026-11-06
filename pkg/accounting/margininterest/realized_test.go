package margininterest

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	. "github.com/c9s/bbgo/pkg/testing/testhelper"
	"github.com/c9s/bbgo/pkg/types"
)

func interestRow(asset, interest, priceInUSDT string, at time.Time) types.MarginInterest {
	return types.MarginInterest{
		Asset:       asset,
		Interest:    Number(interest),
		PriceInUSDT: Number(priceInUSDT),
		Time:        types.Time(at),
	}
}

// bnbPaid is an hourly charge on principal of raw at a daily rate of 0.00024, paid in BNB
func bnbPaid(raw, principal, bnbInterest, bnbPrice string, at time.Time) types.MarginInterest {
	return types.MarginInterest{
		Asset:        "BNB",
		RawAsset:     raw,
		InterestType: "PERIODIC_CONVERTED",
		Principle:    Number(principal),
		InterestRate: Number("0.00024"),
		Interest:     Number(bnbInterest),
		PriceInUSDT:  Number(bnbPrice),
		Time:         types.Time(at),
	}
}

func TestSumRealized(t *testing.T) {
	h10 := time.Date(2026, 11, 15, 10, 0, 0, 0, time.UTC)
	h11 := h10.Add(time.Hour)
	btcPrice := fixedPrices(map[string]string{"BTC": "62000"})

	twoBTC := []types.MarginInterest{
		interestRow("BTC", "0.00000209", "60000", h10),
		interestRow("BTC", "0.00000209", "61000", h11),
	}

	tests := []struct {
		name        string
		rows        []types.MarginInterest
		mode        PriceMode
		pricer      PriceFunc
		wantUSDT    float64
		wantMissing []string
		check       func(t *testing.T, r Realized)
	}{
		{name: "charged", rows: twoBTC, mode: PriceModeCharged, pricer: btcPrice, wantUSDT: 0.25289},
		{name: "empty mode is charged", rows: twoBTC, mode: "", pricer: btcPrice, wantUSDT: 0.25289},
		{name: "current", rows: twoBTC, mode: PriceModeCurrent, pricer: btcPrice, wantUSDT: 0.25916},
		{
			name: "usdt is always 1", mode: PriceModeCharged,
			rows:     []types.MarginInterest{interestRow("USDT", "0.00417", "0", h10)},
			wantUSDT: 0.00417,
		},
		{
			name: "unpriced row", mode: PriceModeCharged,
			rows: []types.MarginInterest{
				interestRow("BTC", "0.00000209", "60000", h10),
				interestRow("BTC", "0.00000209", "0", h11),
			},
			wantUSDT: 0.1254, wantMissing: []string{"BTC"},
			check: func(t *testing.T, r Realized) {
				assert.InDelta(t, 0.00000418, r.Assets["BTC"].Interest.Float64(), tol)
				assert.False(t, r.Assets["BTC"].PriceOK)
			},
		},
		{
			name: "duplicate rows count once", mode: PriceModeCharged,
			rows: []types.MarginInterest{
				interestRow("BTC", "0.00000209", "60000", h10),
				interestRow("BTC", "0.00000209", "60000", h10),
			},
			wantUSDT: 0.1254,
			check: func(t *testing.T, r Realized) {
				assert.Equal(t, 1, r.Assets["BTC"].Rows)
			},
		},
		{
			name: "current price unknown", rows: twoBTC, mode: PriceModeCurrent,
			pricer:   func(string) (fixedpoint.Value, bool) { return fixedpoint.Zero, false },
			wantUSDT: 0, wantMissing: []string{"BTC"},
		},
		{name: "no rows", mode: PriceModeCharged, wantUSDT: 0},
		{
			// one hour of BNB deduction: a row per borrowed asset, all with Asset BNB
			name: "bnb-paid rows of the same hour all count", mode: PriceModeCharged,
			rows: []types.MarginInterest{
				bnbPaid("BTC", "0.5", "0.000003", "600", h10),
				bnbPaid("ETH", "10", "0.000001", "600", h10),
				bnbPaid("ETH", "10", "0.000001", "600", h10), // a copy
			},
			wantUSDT: 0.0024,
			check: func(t *testing.T, r Realized) {
				assert.NotContains(t, r.Assets, "BNB")
				btc := r.Assets["BTC"]
				assert.Equal(t, 1, btc.Rows)
				// nominal 0.5 x 0.00024 / 24, charged in BNB
				assert.InDelta(t, 0.000005, btc.Interest.Float64(), tol)
				assert.InDelta(t, 0.000003, btc.Charged["BNB"].Float64(), tol)
				assert.InDelta(t, 0.0018, btc.USDT.Float64(), tol)
				assert.Equal(t, 1, r.Assets["ETH"].Rows)
				assert.InDelta(t, 0.0001, r.Assets["ETH"].Interest.Float64(), tol)
			},
		},
		{
			// some hours paid in the asset, some in BNB: both amounts are kept apart
			name: "mixed payment, current price", mode: PriceModeCurrent,
			rows: []types.MarginInterest{
				interestRow("BTC", "0.00000209", "0", h10),
				bnbPaid("BTC", "0.5", "0.000003", "0", h11),
			},
			pricer:   fixedPrices(map[string]string{"BTC": "62000", "BNB": "600"}),
			wantUSDT: 0.00000209*62000 + 0.0018,
			check: func(t *testing.T, r Realized) {
				btc := r.Assets["BTC"]
				assert.InDelta(t, 0.00000209+0.000005, btc.Interest.Float64(), tol)
				assert.InDelta(t, 0.00000209, btc.Charged["BTC"].Float64(), tol)
				assert.InDelta(t, 0.000003, btc.Charged["BNB"].Float64(), tol)
				assert.True(t, btc.PriceOK)
			},
		},
		{
			name: "current price of the paid asset unknown", mode: PriceModeCurrent,
			rows:     []types.MarginInterest{bnbPaid("BTC", "0.5", "0.000003", "0", h10)},
			pricer:   fixedPrices(map[string]string{"BTC": "62000"}),
			wantUSDT: 0, wantMissing: []string{"BTC"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := SumRealized(tt.rows, tt.mode, tt.pricer)
			assert.InDelta(t, tt.wantUSDT, r.TotalUSDT.Float64(), tol)
			assert.Equal(t, tt.wantMissing, r.MissingPrice)
			assert.Equal(t, tt.mode.OrDefault(), r.Mode)
			if tt.check != nil {
				tt.check(t, r)
			}
		})
	}
}

func TestPriceModeValidate(t *testing.T) {
	assert.NoError(t, PriceMode("").Validate())
	assert.NoError(t, PriceModeCurrent.Validate())
	assert.NoError(t, PriceModeCharged.Validate())
	assert.Error(t, PriceMode("historical").Validate())
}
