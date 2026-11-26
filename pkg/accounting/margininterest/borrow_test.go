package margininterest

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	. "github.com/c9s/bbgo/pkg/testing/testhelper"
)

func TestBorrowCharges(t *testing.T) {
	// the charge at the borrow, plus every top-of-hour charge that is left
	assert.Equal(t, 374, BorrowCharges(373))
	// in the last hour of the window the borrow still pays its own charge
	assert.Equal(t, 1, BorrowCharges(0))
	assert.Equal(t, 1, BorrowCharges(-5))
}

func TestFitBorrow(t *testing.T) {
	// 1 unit at 0.00001 per hour over 100 charges at a price of 2000 costs 2 USDT
	rate, price, charges := Number("0.00001"), Number(2000), 100

	cases := []struct {
		name               string
		quantity, headroom string
		want               string
	}{
		{name: "all of it fits", quantity: "5", headroom: "20", want: "5"},
		// landing exactly on the headroom is within it
		{name: "exactly the headroom", quantity: "5", headroom: "10", want: "5"},
		{name: "half of it fits", quantity: "5", headroom: "5", want: "2.5"},
		{name: "a small part fits", quantity: "1000", headroom: "1", want: "0.5"},
		{name: "no headroom", quantity: "5", headroom: "0", want: "0"},
		{name: "over the budget already", quantity: "5", headroom: "-3", want: "0"},
		{name: "nothing to borrow", quantity: "0", headroom: "10", want: "0"},
		{name: "a negative quantity", quantity: "-1", headroom: "10", want: "0"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := FitBorrow(Number(c.quantity), Number(c.headroom), rate, price, charges)
			assert.InDelta(t, Number(c.want).Float64(), got.Float64(), tol)

			// what is returned never costs more than the headroom
			if headroom := Number(c.headroom); headroom.Sign() > 0 {
				cost := ProjectInterest(got, rate, charges).Mul(price)
				assert.LessOrEqual(t, cost.Float64(), headroom.Float64()+tol)
			}
		})
	}
}

// A cost that cannot be computed is not this function's to refuse: the caller
// checks the rate and the price before it asks.
func TestFitBorrow_NoCost(t *testing.T) {
	assert.Equal(t, Number(5), FitBorrow(Number(5), Number(1), fixedpoint.Zero, Number(2000), 100))
	assert.Equal(t, Number(5), FitBorrow(Number(5), Number(1), Number("0.00001"), fixedpoint.Zero, 100))
	assert.Equal(t, Number(5), FitBorrow(Number(5), Number(1), Number("0.00001"), Number(2000), 0))
}

// A very cheap asset: the cost of the whole quantity is far below the headroom
func TestFitBorrow_TinyRateAndPrice(t *testing.T) {
	got := FitBorrow(Number(1000000), Number(10), Number("0.00000105"), Number("0.0001"), 744)
	assert.Equal(t, Number(1000000), got)
}
