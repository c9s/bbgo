package grid2

import (
	"fmt"
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type GridProfit struct {
	Currency string           `json:"currency"`
	Profit   fixedpoint.Value `json:"profit"`
	Time     time.Time        `json:"time"`
	Order    types.Order      `json:"order"`
}

func (p *GridProfit) String() string {
	return fmt.Sprintf("GRID PROFIT: %f %s @ %s orderID %d", p.Profit.Float64(), p.Currency, p.Time.String(), p.Order.OrderID)
}
