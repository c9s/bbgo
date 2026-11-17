package margininterest

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
)

// BudgetStatus is how the interest of a month stands against its budget
type BudgetStatus string

const (
	// BudgetStatusExceeded: the interest already charged is over the budget.
	// Final for the month, whatever happens to the principal.
	BudgetStatusExceeded BudgetStatus = "exceeded"

	// BudgetStatusProjectedExceeded: the interest charged so far is within the
	// budget, but keeping the current principal to the month end goes over it.
	// A forecast: repaying can still avoid it.
	BudgetStatusProjectedExceeded BudgetStatus = "projected_exceeded"

	// BudgetStatusOK: within the budget, and the estimate is complete
	BudgetStatusOK BudgetStatus = "ok"

	// BudgetStatusUnknown: the figures are within the budget but incomplete, so
	// the true ones may be over it. It must not be read as ok.
	BudgetStatusUnknown BudgetStatus = "unknown"
)

// BudgetStatuses lists every status, most severe first
var BudgetStatuses = []BudgetStatus{
	BudgetStatusExceeded,
	BudgetStatusProjectedExceeded,
	BudgetStatusUnknown,
	BudgetStatusOK,
}

// BudgetEvaluation is the interest of a month compared against its budget, in USDT
type BudgetEvaluation struct {
	BudgetUSDT fixedpoint.Value

	// RealizedUSDT is the interest charged so far this month
	RealizedUSDT fixedpoint.Value

	// MonthUSDT is RealizedUSDT plus the projection to the month end
	MonthUSDT fixedpoint.Value

	// Complete is false when RealizedUSDT or MonthUSDT may be missing a part.
	// They are then lower bounds.
	Complete bool

	// RemainingUSDT is BudgetUSDT - MonthUSDT, negative when over the budget
	RemainingUSDT fixedpoint.Value

	// UsedRatio is MonthUSDT / BudgetUSDT, 0 without a positive budget
	UsedRatio fixedpoint.Value

	// Exceeded: RealizedUSDT alone is over the budget
	Exceeded bool

	// ProjectedExceeded: MonthUSDT is over the budget. Exceeded implies it,
	// since a projection is never negative.
	ProjectedExceeded bool
}

// EvaluateBudget compares the realized and the month (realized + projected)
// interest against budget. complete says whether both figures are the full
// ones; when it is false they are lower bounds.
//
// Both comparisons are strict: landing exactly on the budget is within it.
// The caller evaluates only a positive budget; no budget is not a zero budget.
func EvaluateBudget(budget, realizedUSDT, monthUSDT fixedpoint.Value, complete bool) BudgetEvaluation {
	e := BudgetEvaluation{
		BudgetUSDT:        budget,
		RealizedUSDT:      realizedUSDT,
		MonthUSDT:         monthUSDT,
		Complete:          complete,
		RemainingUSDT:     budget.Sub(monthUSDT),
		UsedRatio:         fixedpoint.Zero,
		Exceeded:          realizedUSDT.Compare(budget) > 0,
		ProjectedExceeded: monthUSDT.Compare(budget) > 0,
	}

	if budget.Sign() > 0 {
		e.UsedRatio = monthUSDT.Div(budget)
	}

	return e
}

// Status reduces the evaluation to one status. An incomplete evaluation is a
// lower bound: over the budget still counts, under the budget is unknown.
func (e BudgetEvaluation) Status() BudgetStatus {
	switch {
	case e.Exceeded:
		return BudgetStatusExceeded
	case e.ProjectedExceeded:
		return BudgetStatusProjectedExceeded
	case e.Complete:
		return BudgetStatusOK
	default:
		return BudgetStatusUnknown
	}
}
