package grid2

import (
	"fmt"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func debugGrid(grid *Grid, book *bbgo.ActiveOrderBook) {
	fmt.Println("================== GRID ORDERS ==================")

	pins := grid.Pins
	missingPins := scanMissingPinPrices(book, pins)
	missing := len(missingPins)

	for i := len(pins) - 1; i >= 0; i-- {
		pin := pins[i]
		price := fixedpoint.Value(pin)

		fmt.Printf("%s -> ", price.String())

		existingOrder := book.Lookup(func(o types.Order) bool {
			return o.Price.Eq(price)
		})

		if existingOrder != nil {
			fmt.Printf("%s", existingOrder.String())

			switch existingOrder.Status {
			case types.OrderStatusFilled:
				fmt.Printf(" | 🔧")
			case types.OrderStatusCanceled:
				fmt.Printf(" | 🔄")
			default:
				fmt.Printf(" | ✅")
			}
		} else {
			fmt.Printf("ORDER MISSING ⚠️ ")
			if missing == 1 {
				fmt.Printf(" COULD BE EMPTY SLOT")
			}
		}
		fmt.Printf("\n")
	}
	fmt.Println("================== END OF GRID ORDERS ===================")
}
