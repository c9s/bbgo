package margininterest

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	. "github.com/c9s/bbgo/pkg/testing/testhelper"
	"github.com/c9s/bbgo/pkg/types"
)

// interestRow is a valued row; priceInUSDT "" leaves it unpriced
func interestRow(asset, interest, priceInUSDT string, at time.Time) types.MarginInterestValuation {
	row := types.MarginInterestValuation{
		PaidAsset:     asset,
		BorrowedAsset: asset,
		Interest:      Number(interest),
		Time:          types.Time(at),
	}
	if priceInUSDT != "" {
		price := Number(priceInUSDT)
		usdt := row.Interest.Mul(price)
		row.PriceInUSDT, row.InterestUSDT = &price, &usdt
	}
	return row
}

func TestSumRealized(t *testing.T) {
	h10 := time.Date(2026, 11, 15, 10, 0, 0, 0, time.UTC)
	h11 := h10.Add(time.Hour)
	btcPrice := fixedPrices(map[string]string{"BTC": "62000"})

	twoBTC := []types.MarginInterestValuation{
		interestRow("BTC", "0.00000209", "60000", h10),
		interestRow("BTC", "0.00000209", "61000", h11),
	}

	tests := []struct {
		name        string
		rows        []types.MarginInterestValuation
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
			rows:     []types.MarginInterestValuation{interestRow("USDT", "0.00417", "", h10)},
			wantUSDT: 0.00417,
		},
		{
			name: "unpriced row", mode: PriceModeCharged,
			rows: []types.MarginInterestValuation{
				interestRow("BTC", "0.00000209", "60000", h10),
				interestRow("BTC", "0.00000209", "", h11),
			},
			wantUSDT: 0.1254, wantMissing: []string{"BTC"},
			check: func(t *testing.T, r Realized) {
				assert.InDelta(t, 0.00000418, r.Assets["BTC"].Interest.Float64(), tol)
				assert.False(t, r.Assets["BTC"].PriceOK)
			},
		},
		{
			// e.g. an ON_BORROW and a PERIODIC charge in the same hour
			name: "charges of the same hour all count", mode: PriceModeCharged,
			rows: []types.MarginInterestValuation{
				interestRow("BTC", "0.00000209", "60000", h10),
				interestRow("BTC", "0.00000209", "60000", h10),
			},
			wantUSDT: 0.2508,
			check: func(t *testing.T, r Realized) {
				assert.Equal(t, 2, r.Assets["BTC"].Rows)
			},
		},
		{
			// the price of the paid asset, BNB, not of the borrowed one
			name: "converted row is valued in the paid asset", mode: PriceModeCharged,
			rows: []types.MarginInterestValuation{func() types.MarginInterestValuation {
				row := interestRow("BNB", "0.0001", "600", h10)
				row.BorrowedAsset, row.Converted = "BTC", true
				return row
			}()},
			wantUSDT: 0.06,
			check: func(t *testing.T, r Realized) {
				assert.Contains(t, r.Assets, "BNB")
				assert.NotContains(t, r.Assets, "BTC")
			},
		},
		{
			name: "current price unknown", rows: twoBTC, mode: PriceModeCurrent,
			pricer:   func(string) (fixedpoint.Value, bool) { return fixedpoint.Zero, false },
			wantUSDT: 0, wantMissing: []string{"BTC"},
		},
		{name: "no rows", mode: PriceModeCharged, wantUSDT: 0},
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
