package autoborrow

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	. "github.com/c9s/bbgo/pkg/testing/testhelper"
	"github.com/c9s/bbgo/pkg/types"
)

const delta = 1e-6

func Test_hoursUntilEndOfMonth(t *testing.T) {
	tests := []struct {
		name string
		now  time.Time
		want float64
	}{
		{"beginning of october", time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), 744},
		{"middle of october", time.Date(2026, 10, 8, 0, 0, 0, 0, time.UTC), 576},
		{"last day of december", time.Date(2026, 12, 31, 12, 30, 0, 0, time.UTC), 11.5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.InDelta(t, tt.want, hoursUntilEndOfMonth(tt.now).Float64(), delta)
		})
	}
}

func Test_hoursInMonth(t *testing.T) {
	assert.InDelta(t, 744.0, hoursInMonth(time.Date(2026, 10, 8, 3, 0, 0, 0, time.UTC)).Float64(), delta)
	assert.InDelta(t, 720.0, hoursInMonth(time.Date(2026, 11, 30, 23, 0, 0, 0, time.UTC)).Float64(), delta)
	assert.InDelta(t, 672.0, hoursInMonth(time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)).Float64(), delta)
	assert.InDelta(t, 696.0, hoursInMonth(time.Date(2024, 2, 15, 0, 0, 0, 0, time.UTC)).Float64(), delta)
}

func debtBalance(asset string, borrowed, interest float64) types.Balance {
	return types.Balance{
		Currency: asset,
		Borrowed: Number(borrowed),
		Interest: Number(interest),
	}
}

func hourlyRate(asset string, rate float64) *types.MarginNextHourlyInterestRate {
	return &types.MarginNextHourlyInterestRate{
		Asset:      asset,
		HourlyRate: Number(rate),
	}
}

func Test_estimateInterest(t *testing.T) {
	// 24 hours to the end of october, 744 hours in october
	now := time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)

	debts := types.BalanceMap{
		"BTC":  debtBalance("BTC", 0.9, 0.1),
		"USDT": debtBalance("USDT", 1000, 0),
		"ETH":  debtBalance("ETH", 1, 0),    // no rate
		"DOGE": debtBalance("DOGE", 100, 0), // no price
		"BNB":  debtBalance("BNB", 0, 0),    // no debt
	}

	rates := types.MarginNextHourlyInterestRateMap{
		"BTC":  hourlyRate("BTC", 0.00001),
		"USDT": hourlyRate("USDT", 0.00002),
		"DOGE": hourlyRate("DOGE", 0.0001),
		"BNB":  hourlyRate("BNB", 0.0001),
	}

	prices := map[string]fixedpoint.Value{
		"BTC": Number(60000),
		"ETH": Number(3000),
		"BNB": Number(500),
	}

	est := estimateInterest(now, "USDT", debts, rates, prices)

	assert.InDelta(t, 24.0, est.HoursToMonthEnd.Float64(), delta)
	assert.InDelta(t, 744.0, est.HoursInMonth.Float64(), delta)
	assert.Equal(t, []string{"ETH"}, est.MissingRates)
	assert.Equal(t, []string{"DOGE"}, est.MissingPrices)

	require.Len(t, est.Assets, 2)

	btc := est.Assets[0]
	assert.Equal(t, "BTC", btc.Asset)
	assert.InDelta(t, 1.0, btc.Debt.Float64(), delta)
	assert.InDelta(t, 60000.0, btc.DebtValue.Float64(), delta)
	assert.InDelta(t, 0.6, btc.HourlyInterest.Float64(), delta)
	assert.InDelta(t, 14.4, btc.InterestToMonthEnd.Float64(), delta)
	assert.InDelta(t, 446.4, btc.MonthlyInterest.Float64(), delta)

	usdt := est.Assets[1]
	assert.Equal(t, "USDT", usdt.Asset)
	assert.InDelta(t, 1.0, usdt.Price.Float64(), delta)
	assert.InDelta(t, 1000.0, usdt.DebtValue.Float64(), delta)
	assert.InDelta(t, 0.02, usdt.HourlyInterest.Float64(), delta)
	assert.InDelta(t, 0.48, usdt.InterestToMonthEnd.Float64(), delta)
	assert.InDelta(t, 14.88, usdt.MonthlyInterest.Float64(), delta)

	assert.InDelta(t, 61000.0, est.TotalDebtValue.Float64(), delta)
	assert.InDelta(t, 0.62, est.TotalHourlyInterest.Float64(), delta)
	assert.InDelta(t, 14.88, est.TotalInterestToMonthEnd.Float64(), delta)
	assert.InDelta(t, 461.28, est.TotalMonthlyInterest.Float64(), delta)
}

func Test_estimateInterest_noDebt(t *testing.T) {
	est := estimateInterest(time.Date(2026, 10, 8, 0, 0, 0, 0, time.UTC), "USDT", types.BalanceMap{}, nil, nil)
	assert.Empty(t, est.Assets)
	assert.True(t, est.TotalMonthlyInterest.IsZero())
	assert.True(t, est.TotalInterestToMonthEnd.IsZero())
}

func Test_maxBorrowWithinBudget(t *testing.T) {
	// 1 BTC costs 60000 * 0.00001 * 744 = 446.4 USDT per month
	t.Run("within budget", func(t *testing.T) {
		q := maxBorrowWithinBudget(Number(53.6), Number(500), Number(60000), Number(0.00001), Number(744))
		assert.InDelta(t, 1.0, q.Float64(), delta)
	})

	t.Run("budget used up", func(t *testing.T) {
		q := maxBorrowWithinBudget(Number(500), Number(500), Number(60000), Number(0.00001), Number(744))
		assert.True(t, q.IsZero())

		q = maxBorrowWithinBudget(Number(600), Number(500), Number(60000), Number(0.00001), Number(744))
		assert.True(t, q.IsZero())
	})

	t.Run("zero interest rate", func(t *testing.T) {
		q := maxBorrowWithinBudget(Number(100), Number(500), Number(60000), Number(0), Number(744))
		assert.Equal(t, fixedpoint.PosInf, q)
	})
}

func Test_interestBudgetGuard(t *testing.T) {
	now := time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)
	debts := types.BalanceMap{
		"USDT": debtBalance("USDT", 1000, 0), // 14.88 USDT per month
	}
	rates := types.MarginNextHourlyInterestRateMap{
		"BTC":  hourlyRate("BTC", 0.00001),
		"USDT": hourlyRate("USDT", 0.00002),
		"ETH":  hourlyRate("ETH", 0.00001),
	}
	prices := map[string]fixedpoint.Value{
		"BTC": Number(60000),
	}

	est := estimateInterest(now, "USDT", debts, rates, prices)

	// budget leaves 446.4 USDT, which is exactly 1 BTC for a month
	guard := newInterestBudgetGuard(Number(14.88+446.4), est, rates, prices)
	assert.False(t, guard.Exceeded())

	q, err := guard.Limit("BTC", Number(0.5))
	require.NoError(t, err)
	assert.InDelta(t, 0.5, q.Float64(), delta, "quantity within the budget is not changed")

	q, err = guard.Limit("BTC", Number(2))
	require.NoError(t, err)
	assert.InDelta(t, 1.0, q.Float64(), delta, "quantity is reduced to fit the budget")

	_, err = guard.Limit("DOGE", Number(1))
	assert.Error(t, err, "unknown rate")

	_, err = guard.Limit("ETH", Number(1))
	assert.Error(t, err, "unknown price")

	guard.AddBorrow("BTC", Number(1))
	assert.InDelta(t, 14.88+446.4, est.TotalMonthlyInterest.Float64(), delta)
	assert.True(t, guard.Exceeded())

	q, err = guard.Limit("USDT", Number(100))
	require.NoError(t, err)
	assert.True(t, q.IsZero(), "nothing can be borrowed once the budget is used up")
}
