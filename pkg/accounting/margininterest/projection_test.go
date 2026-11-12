package margininterest

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	. "github.com/c9s/bbgo/pkg/testing/testhelper"
	"github.com/c9s/bbgo/pkg/types"
)

func TestProjectInterest(t *testing.T) {
	tests := []struct {
		name      string
		principal string
		hours     int
		rate      string
		want      float64
	}{
		{"usdt", "1000", 720, "0.00000417", 3.0024},
		{"btc", "2", 720, "0.00000125", 0.0018},
		// principal x rate first would give 0.00154752
		{"small principal", "0.5", 744, "0.00000417", 0.00155124},
		// hours x rate first would give 781.19 in non-dnum
		{"hours x rate is not exact", "1000000", 744, "0.00000105", 781.2},
		// non-dnum: 7.19999999
		{"float truncation", "10000", 720, "0.000001", 7.2},
		{"zero principal", "0", 720, "0.00000417", 0},
		{"zero hours", "1000", 0, "0.00000417", 0},
		{"zero rate", "1000", 720, "0", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ProjectInterest(Number(tt.principal), Number(tt.rate), tt.hours)
			assert.InDelta(t, tt.want, got.Float64(), tol)
		})
	}
}

func TestProjectInterest_HugePrincipalDoesNotOverflow(t *testing.T) {
	// non-dnum: principal x hours overflows int64, so the fallback order is used
	got := ProjectInterest(Number("200000000"), Number("0.00000417"), 744)
	assert.False(t, got.IsInf())
	assert.InDelta(t, 620496.0, got.Float64(), 1e-3)
}

func rateMap(rates map[string]string) types.MarginNextHourlyInterestRateMap {
	m := types.MarginNextHourlyInterestRateMap{}
	for asset, r := range rates {
		if r == "" {
			m[asset] = nil
			continue
		}
		m[asset] = &types.MarginNextHourlyInterestRate{Asset: asset, HourlyRate: Number(r)}
	}
	return m
}

func fixedPrices(prices map[string]string) PriceFunc {
	return func(asset string) (fixedpoint.Value, bool) {
		p, ok := prices[asset]
		if !ok {
			return fixedpoint.Zero, false
		}
		return Number(p), true
	}
}

func debt(asset, borrowed, interest string) types.Balance {
	b := types.NewZeroBalance(asset)
	b.Borrowed = Number(borrowed)
	b.Interest = Number(interest)
	return b
}

func TestProjectAssets(t *testing.T) {
	prices := fixedPrices(map[string]string{"BTC": "60000", "ETH": "3000"})

	t.Run("mixed", func(t *testing.T) {
		debts := types.BalanceMap{
			"BTC":  debt("BTC", "0.5", "0.0001"),
			"USDT": debt("USDT", "1000", "0"),
			"ETH":  debt("ETH", "2", "0"),
			"XRP":  debt("XRP", "10", "0"),
			"DOGE": debt("DOGE", "1000", "0"),
			"SOL":  debt("SOL", "0", "0.01"),
		}
		rates := rateMap(map[string]string{
			"BTC": "0.00000417", "USDT": "0.00000417", "XRP": "", "DOGE": "0.00000417",
		})

		p := ProjectAssets([]string{"BTC", "USDT", "ETH", "XRP", "DOGE", "SOL"}, debts, rates, prices, 744)

		assert.InDelta(t, 0.00155124, p.Assets["BTC"].Interest.Float64(), tol)
		assert.InDelta(t, 93.0744, p.Assets["BTC"].USDT.Float64(), tol)
		assert.InDelta(t, 3.10248, p.Assets["USDT"].USDT.Float64(), tol)

		assert.InDelta(t, 3.10248, p.Assets["DOGE"].Interest.Float64(), tol)
		assert.False(t, p.Assets["DOGE"].PriceOK)
		assert.True(t, p.Assets["DOGE"].USDT.IsZero())

		assert.NotContains(t, p.Assets, "ETH")
		// SOL has accrued interest but no principal: listed with zero interest
		assert.True(t, p.Assets["SOL"].Interest.IsZero())
		assert.True(t, p.Assets["SOL"].PriceOK)
		assert.Equal(t, []string{"ETH", "XRP"}, p.MissingRate)
		assert.Equal(t, []string{"DOGE"}, p.MissingPrice)
		assert.InDelta(t, 96.17688, p.TotalUSDT.Float64(), tol)
		assert.False(t, p.Complete())
	})

	t.Run("complete", func(t *testing.T) {
		debts := types.BalanceMap{
			"BTC":  debt("BTC", "0.5", "0.0001"),
			"USDT": debt("USDT", "1000", "0"),
		}
		rates := rateMap(map[string]string{"BTC": "0.00000417", "USDT": "0.00000417"})

		p := ProjectAssets([]string{"BTC", "USDT"}, debts, rates, prices, 744)
		assert.InDelta(t, 96.17688, p.TotalUSDT.Float64(), tol)
		assert.True(t, p.Complete())
	})

	t.Run("last hour of the month", func(t *testing.T) {
		debts := types.BalanceMap{"USDT": debt("USDT", "1000", "0")}
		rates := rateMap(map[string]string{"USDT": "0.00000417"})

		p := ProjectAssets([]string{"USDT"}, debts, rates, nil, 0)
		assert.True(t, p.Assets["USDT"].Interest.IsZero())
		assert.True(t, p.Complete())
	})

	t.Run("assets without debt are listed", func(t *testing.T) {
		debts := types.BalanceMap{
			"USDT": debt("USDT", "1000", "0"),
			// a debt outside the asset list is not projected
			"BTC": debt("BTC", "0.5", "0"),
		}
		// PEPE has no price and no rate, ETH has a rate but no debt
		rates := rateMap(map[string]string{"USDT": "0.00000417", "ETH": "0.000002"})

		p := ProjectAssets([]string{"ETH", "PEPE", "USDT", "ETH"}, debts, rates, nil, 744)
		assert.ElementsMatch(t, []string{"ETH", "PEPE", "USDT"}, keysOf(p.Assets))
		assert.NotContains(t, p.Assets, "BTC")

		eth := p.Assets["ETH"]
		assert.True(t, eth.RateOK)
		assert.InDelta(t, 0.000002, eth.HourlyRate.Float64(), tol)
		assert.True(t, eth.Principal.IsZero())
		assert.True(t, eth.Interest.IsZero())
		assert.True(t, eth.PriceOK)

		pepe := p.Assets["PEPE"]
		assert.False(t, pepe.RateOK)
		assert.True(t, pepe.Interest.IsZero())

		assert.InDelta(t, 3.10248, p.TotalUSDT.Float64(), tol)
		assert.True(t, p.Complete())
	})
}

func keysOf(m map[string]AssetProjection) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
