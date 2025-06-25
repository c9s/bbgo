package tradingdesk

import (
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type OpenPositionParam struct {
	Symbol          string           `json:"symbol"`
	Confidence      fixedpoint.Value `json:"confidence"`
	Side            types.SideType   `json:"side"`
	Quantity        fixedpoint.Value `json:"quantity"`
	StopLossPrice   fixedpoint.Value `json:"stopLossPrice"`
	TakeProfitPrice fixedpoint.Value `json:"takeProfitPrice"`
	TimeToLive      time.Duration    `json:"timeToLive"`
}
