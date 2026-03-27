package grid2

import (
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func debugGrid(logger logrus.FieldLogger, grid *Grid, book *bbgo.ActiveOrderBook) {
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
			sb.WriteString(existingOrder.String())

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

	logger.Infoln(sb.String())
}

func (s *Strategy) debugGridOrders(submitOrders []types.SubmitOrder, lastPrice fixedpoint.Value) {
	if !s.Debug {
		return
	}

	var sb strings.Builder

	sb.WriteString("GRID ORDERS [\n")
	for i, order := range submitOrders {
		if i > 0 && lastPrice.Compare(order.Price) >= 0 && lastPrice.Compare(submitOrders[i-1].Price) <= 0 {
			sb.WriteString(fmt.Sprintf("  - LAST PRICE: %f\n", lastPrice.Float64()))
		}

		sb.WriteString("  - " + order.String() + "\n")
	}
	sb.WriteString("] END OF GRID ORDERS")

	s.logger.Info(sb.String())
}

func (s *Strategy) debugOrders(desc string, orders []types.Order) {
	if !s.Debug {
		return
	}

	var sb strings.Builder

	if desc == "" {
		desc = "ORDERS"
	}

	sb.WriteString(desc + " [\n")
	for i, order := range orders {
		sb.WriteString(fmt.Sprintf("  - %d) %s\n", i, order.String()))
	}
	sb.WriteString("]")

	s.logger.Info(sb.String())
}

func (s *Strategy) debugLog(format string, args ...interface{}) {
	if !s.Debug {
		return
	}

	s.logger.Infof(format, args...)
}
