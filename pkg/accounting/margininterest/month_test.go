package margininterest

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	. "github.com/c9s/bbgo/pkg/testing/testhelper"
	"github.com/c9s/bbgo/pkg/types"
)

func TestMonthByAsset(t *testing.T) {
	h10 := time.Date(2026, 11, 15, 10, 0, 0, 0, time.UTC)

	debts := types.BalanceMap{
		"BTC":  {Currency: "BTC", Borrowed: Number(0.5)},
		"USDT": {Currency: "USDT", Borrowed: Number(1000)},
		"DOGE": {Currency: "DOGE", Borrowed: Number(100)},
		"PEPE": {Currency: "PEPE", Borrowed: Number(1e6)},
	}
	rates := types.MarginNextHourlyInterestRateMap{
		"BTC":  {Asset: "BTC", HourlyRate: Number("0.000001")},
		"USDT": {Asset: "USDT", HourlyRate: Number("0.000002")},
		"PEPE": {Asset: "PEPE", HourlyRate: Number("0.000001")},
		// DOGE has no rate
	}
	pricer := fixedPrices(map[string]string{"BTC": "60000"})
	p := ProjectAssets([]string{"BTC", "DOGE", "PEPE", "USDT"}, debts, rates, pricer, 100)

	r := SumRealized([]types.MarginInterest{
		interestRow("BTC", "0.00001", "60000", h10),
		// ETH was repaid: realized only
		interestRow("ETH", "0.001", "3000", h10),
		interestRow("USDT", "0.5", "0", h10),
	}, PriceModeCharged, pricer)

	months := MonthByAsset(p, r)
	assert.Len(t, months, 5)

	// BTC: 0.5 x 100 x 0.000001 = 0.00005 projected + 0.00001 realized
	btc := months["BTC"]
	assert.InDelta(t, 0.00005, btc.Projected.Float64(), tol)
	assert.InDelta(t, 0.00001, btc.Realized.Float64(), tol)
	assert.InDelta(t, 0.00006, btc.Total.Float64(), tol)
	assert.InDelta(t, 3.6, btc.TotalUSDT.Float64(), 1e-3)
	assert.True(t, btc.Complete())

	// USDT: 1000 x 100 x 0.000002 = 0.2 projected + 0.5 realized, price 1
	usdt := months["USDT"]
	assert.InDelta(t, 0.7, usdt.Total.Float64(), tol)
	assert.InDelta(t, 0.7, usdt.TotalUSDT.Float64(), tol)
	assert.True(t, usdt.Complete())

	eth := months["ETH"]
	assert.True(t, eth.Projected.IsZero())
	assert.InDelta(t, 0.001, eth.Total.Float64(), tol)
	assert.InDelta(t, 3, eth.TotalUSDT.Float64(), tol)
	assert.True(t, eth.Complete())

	// no rate: the amount is a lower bound
	doge := months["DOGE"]
	assert.True(t, doge.MissingRate)
	assert.False(t, doge.AmountComplete())
	assert.True(t, doge.Total.IsZero())

	// no price: the amount is complete, USDT is not
	pepe := months["PEPE"]
	assert.InDelta(t, 100, pepe.Total.Float64(), tol)
	assert.True(t, pepe.TotalUSDT.IsZero())
	assert.True(t, pepe.MissingPrice)
	assert.True(t, pepe.AmountComplete())
	assert.False(t, pepe.Complete())

	// the per-asset USDT adds up to the session totals
	sum := fixedpoint.Zero
	for _, m := range months {
		sum = sum.Add(m.TotalUSDT)
	}
	assert.InDelta(t, p.TotalUSDT.Add(r.TotalUSDT).Float64(), sum.Float64(), tol)
}

func TestAssetMonth_MissingHistory(t *testing.T) {
	m := AssetMonth{Asset: "BTC", MissingHistory: true}
	assert.False(t, m.AmountComplete())
	assert.False(t, m.Complete())
}
