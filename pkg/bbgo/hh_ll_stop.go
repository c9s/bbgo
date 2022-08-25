package bbgo

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type HigherHighLowerLowStopLoss struct {
	Symbol string `json:"symbol"`

	Side types.SideType `json:"side"`

	types.IntervalWindow

	HighLowWindow int `json:"highLowWindow"`

	MaxHighLow int `json:"maxHighLow"`

	MinHighLow int `json:"minHighLow"`

	// ActivationRatio is the trigger condition
	// When the price goes higher (lower for side buy) than this ratio, the stop will be activated.
	ActivationRatio fixedpoint.Value `json:"activationRatio"`

	// DeactivationRatio is the kill condition
	// When the price goes higher (lower for short position) than this ratio, the stop will be deactivated.
	// You can use this to combine several exits
	DeactivationRatio fixedpoint.Value `json:"deactivationRatio"`

	session       *ExchangeSession
	orderExecutor *GeneralOrderExecutor
}
