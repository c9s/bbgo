package grid2

import (
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
