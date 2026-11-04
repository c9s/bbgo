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
		price       PriceFunc
		wantUSDT    float64
		wantMissing []string
		check       func(t *testing.T, r Realized)
	}{
		{name: "charged", rows: twoBTC, mode: PriceModeCharged, price: btcPrice, wantUSDT: 0.25289},
		{name: "empty mode is charged", rows: twoBTC, mode: "", price: btcPrice, wantUSDT: 0.25289},
		{name: "current", rows: twoBTC, mode: PriceModeCurrent, price: btcPrice, wantUSDT: 0.25916},
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
			price:    func(string) (fixedpoint.Value, bool) { return fixedpoint.Zero, false },
			wantUSDT: 0, wantMissing: []string{"BTC"},
		},
		{name: "no rows", mode: PriceModeCharged, wantUSDT: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := SumRealized(tt.rows, tt.mode, tt.price)
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
