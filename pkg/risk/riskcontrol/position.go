package riskcontrol

import (
	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

//go:generate callbackgen -type PositionRiskControl
type PositionRiskControl struct {
	hardLimit fixedpoint.Value
	quantity  fixedpoint.Value

	releasePositionCallbacks []func(quantity fixedpoint.Value, side types.SideType)
}

func NewPositionRiskControl(hardLimit, quantity fixedpoint.Value, tradeCollector *bbgo.TradeCollector) *PositionRiskControl {
	p := &PositionRiskControl{
		hardLimit: hardLimit,
		quantity:  quantity,
	}

	// register position update handler: check if position is over the hard limit
	tradeCollector.OnPositionUpdate(func(position *types.Position) {
		if fixedpoint.Compare(position.Base, hardLimit) > 0 {
			log.Infof("position %f is over hardlimit %f, releasing position...", position.Base.Float64(), hardLimit.Float64())
			p.EmitReleasePosition(position.Base.Sub(hardLimit), types.SideTypeSell)
		} else if fixedpoint.Compare(position.Base, hardLimit.Neg()) < 0 {
			log.Infof("position %f is over hardlimit %f, releasing position...", position.Base.Float64(), hardLimit.Float64())
			p.EmitReleasePosition(position.Base.Neg().Sub(hardLimit), types.SideTypeBuy)
		}
	})

	return p
}

// ModifiedQuantity returns quantity controlled by position risks
// For buy orders, mod quantity = min(hardLimit - position, quantity), limiting by positive position
// For sell orders, mod quantity = min(hardLimit - (-position), quantity), limiting by negative position
func (p *PositionRiskControl) ModifiedQuantity(position fixedpoint.Value) (buyQuantity, sellQuantity fixedpoint.Value) {
	return fixedpoint.Min(p.hardLimit.Sub(position), p.quantity),
		fixedpoint.Min(p.hardLimit.Add(position), p.quantity)
}
