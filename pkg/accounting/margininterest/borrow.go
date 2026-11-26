package margininterest

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
)

// BorrowCharges returns the number of hourly charges a borrow made now pays
// until the end of a window that has remainingHours top-of-hour charges left:
// those, plus the one charged at the moment of the borrow.
//
// The charge at the borrow is counted as a whole hour. That is its upper bound,
// which is the safe side for a limit.
func BorrowCharges(remainingHours int) int {
	if remainingHours < 0 {
		remainingHours = 0
	}
	return remainingHours + 1
}

// FitBorrow returns the largest part of quantity whose interest over the given
// number of hourly charges, valued at price, fits headroomUSDT. It returns
// quantity when all of it fits, and 0 when there is no headroom.
//
// The caller passes a known rate and price: a non-positive one costs nothing
// here, so the whole quantity would fit.
//
// The result is exact to the precision of the arithmetic: in the non-dnum build
// every product is truncated to 8 decimals, so the cost of the result can differ
// from the headroom in the last digit.
func FitBorrow(quantity, headroomUSDT, hourlyRate, price fixedpoint.Value, charges int) fixedpoint.Value {
	if quantity.Sign() <= 0 || headroomUSDT.Sign() <= 0 {
		return fixedpoint.Zero
	}

	cost := ProjectInterest(quantity, hourlyRate, charges).Mul(price)
	if cost.Compare(headroomUSDT) <= 0 {
		return quantity
	}

	// cost is over the headroom, so it is positive and the ratio is below 1
	return quantity.Mul(headroomUSDT.Div(cost))
}
