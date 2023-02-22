package grid2

import (
	"fmt"
	"strings"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func debugGrid(grid *Grid, book *bbgo.ActiveOrderBook) {
	var sb strings.Builder

	sb.WriteString("================== GRID ORDERS ==================\n")

	pins := grid.Pins
	missingPins := scanMissingPinPrices(book, pins)
	missing := len(missingPins)

	for i := len(pins) - 1; i >= 0; i-- {
		pin := pins[i]
		price := fixedpoint.Value(pin)

		sb.WriteString(fmt.Sprintf("%s -> ", price.String()))

		existingOrder := book.Lookup(func(o types.Order) bool {
			return o.Price.Eq(price)
		})

		if existingOrder != nil {
			sb.WriteString(fmt.Sprintf("%s", existingOrder.String()))

			switch existingOrder.Status {
			case types.OrderStatusFilled:
				sb.WriteString(" | 🔧")
			case types.OrderStatusCanceled:
				sb.WriteString(" | 🔄")
			default:
				sb.WriteString(" | ✅")
			}
		} else {
			sb.WriteString("ORDER MISSING ⚠️ ")
			if missing == 1 {
				sb.WriteString(" COULD BE EMPTY SLOT")
			}
		}
		sb.WriteString("\n")
	}

	sb.WriteString("================== END OF GRID ORDERS ===================")

	fmt.Println(sb.String())
}
