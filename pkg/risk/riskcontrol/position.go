package riskcontrol

import (
	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	log "github.com/sirupsen/logrus"
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
	// register position update handler: check if position is over hardlimit
	tradeCollector.OnPositionUpdate(func(position *types.Position) {
		if fixedpoint.Compare(position.Base, hardLimit) > 0 {
			log.Infof("Position %v is over hardlimit %v, releasing:\n", position.Base, hardLimit)
			p.EmitReleasePosition(position.Base.Sub(hardLimit), types.SideTypeSell)
		} else if fixedpoint.Compare(position.Base, hardLimit.Neg()) < 0 {
			log.Infof("Position %v is over hardlimit %v, releasing:\n", position.Base, hardLimit)
			p.EmitReleasePosition(position.Base.Neg().Sub(hardLimit), types.SideTypeBuy)
		}
	})
	return p
}

// ModifiedQuantity returns quantity controlled by position risks
// For buy orders, mod quantity = min(hardlimit - position, quanity), limiting by positive position
// For sell orders, mod quantity = min(hardlimit - (-position), quanity), limiting by negative position
func (p *PositionRiskControl) ModifiedQuantity(position fixedpoint.Value) (buyQuanity, sellQuantity fixedpoint.Value) {
	return fixedpoint.Min(p.hardLimit.Sub(position), p.quantity),
		fixedpoint.Min(p.hardLimit.Add(position), p.quantity)
}
