package dca2

import (
	"github.com/c9s/bbgo/pkg/strategy/dca2/statemachine"
)

const (
	IdleWaiting statemachine.State = iota + 1
	OpenPositionReady
	OpenPositionOrderFilled
	OpenPositionFinished
	TakeProfitReady
)
