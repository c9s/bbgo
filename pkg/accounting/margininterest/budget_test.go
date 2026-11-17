package margininterest

import (
	"testing"

	"github.com/stretchr/testify/assert"

	. "github.com/c9s/bbgo/pkg/testing/testhelper"
)

func TestEvaluateBudget_Status(t *testing.T) {
	cases := []struct {
		name            string
		realized, month string
		complete        bool
		status          BudgetStatus
		exceeded        bool
		projected       bool
	}{
		{name: "under", realized: "40", month: "90", complete: true, status: BudgetStatusOK},
		{name: "nothing charged or projected", realized: "0", month: "0", complete: true, status: BudgetStatusOK},
		// landing exactly on the budget is within it
		{name: "month at the budget", realized: "40", month: "100", complete: true, status: BudgetStatusOK},
		{name: "realized at the budget", realized: "100", month: "100", complete: true, status: BudgetStatusOK},
		{
			name: "month over", realized: "40", month: "100.00000001", complete: true,
			status: BudgetStatusProjectedExceeded, projected: true,
		},
		{
			name: "realized over", realized: "100.00000001", month: "120", complete: true,
			status: BudgetStatusExceeded, exceeded: true, projected: true,
		},
		// an incomplete figure is a lower bound: under is unknown, over still counts
		{name: "incomplete under", realized: "40", month: "90", status: BudgetStatusUnknown},
		{name: "incomplete at the budget", realized: "100", month: "100", status: BudgetStatusUnknown},
		{
			name: "incomplete month over", realized: "40", month: "101",
			status: BudgetStatusProjectedExceeded, projected: true,
		},
		{
			name: "incomplete realized over", realized: "101", month: "101",
			status: BudgetStatusExceeded, exceeded: true, projected: true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := EvaluateBudget(Number(100), Number(c.realized), Number(c.month), c.complete)

			assert.Equal(t, c.status, e.Status())
			assert.Equal(t, c.exceeded, e.Exceeded)
			assert.Equal(t, c.projected, e.ProjectedExceeded)
			assert.Equal(t, c.complete, e.Complete)
		})
	}
}

func TestEvaluateBudget_Figures(t *testing.T) {
	e := EvaluateBudget(Number(200), Number(30), Number(50), true)
	assert.InDelta(t, 200, e.BudgetUSDT.Float64(), tol)
	assert.InDelta(t, 30, e.RealizedUSDT.Float64(), tol)
	assert.InDelta(t, 50, e.MonthUSDT.Float64(), tol)
	assert.InDelta(t, 150, e.RemainingUSDT.Float64(), tol)
	assert.InDelta(t, 0.25, e.UsedRatio.Float64(), tol)

	// over the budget: the remaining goes negative, the ratio over 1
	over := EvaluateBudget(Number(200), Number(30), Number(250), true)
	assert.InDelta(t, -50, over.RemainingUSDT.Float64(), tol)
	assert.InDelta(t, 1.25, over.UsedRatio.Float64(), tol)
}

func TestEvaluateBudget_NoPositiveBudgetHasNoRatio(t *testing.T) {
	e := EvaluateBudget(Number(0), Number(1), Number(2), true)
	assert.True(t, e.UsedRatio.IsZero())
	assert.InDelta(t, -2, e.RemainingUSDT.Float64(), tol)
}

func TestBudgetStatuses_ListEveryStatus(t *testing.T) {
	assert.ElementsMatch(t, []BudgetStatus{
		BudgetStatusExceeded, BudgetStatusProjectedExceeded, BudgetStatusOK, BudgetStatusUnknown,
	}, BudgetStatuses)
}
