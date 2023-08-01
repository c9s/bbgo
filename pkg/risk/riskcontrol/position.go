package riskcontrol

import (
	"context"

	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

// PositionRiskControl controls the position with the given hard limit
// TODO: add a decorator for the order executor and move the order submission logics into the decorator
//
//go:generate callbackgen -type PositionRiskControl
type PositionRiskControl struct {
	orderExecutor bbgo.OrderExecutorExtended

	// hardLimit is the maximum base position you can hold
	hardLimit fixedpoint.Value

	// sliceQuantity is the maximum quantity of the order you want to place.
	// only used in the ModifiedQuantity method
	sliceQuantity fixedpoint.Value

	releasePositionCallbacks []func(quantity fixedpoint.Value, side types.SideType)
}

func NewPositionRiskControl(orderExecutor bbgo.OrderExecutorExtended, hardLimit, quantity fixedpoint.Value) *PositionRiskControl {
	control := &PositionRiskControl{
		orderExecutor: orderExecutor,
		hardLimit:     hardLimit,
		sliceQuantity: quantity,
	}

	control.OnReleasePosition(func(quantity fixedpoint.Value, side types.SideType) {
		pos := orderExecutor.Position()
		submitOrder := types.SubmitOrder{
			Symbol:   pos.Symbol,
			Market:   pos.Market,
			Side:     side,
			Type:     types.OrderTypeMarket,
			Quantity: quantity,
		}

		log.Infof("RiskControl: position limit exceeded, submitting order to reduce position: %+v", submitOrder)
		createdOrders, err := orderExecutor.SubmitOrders(context.Background(), submitOrder)
		if err != nil {
			log.WithError(err).Errorf("failed to submit orders")
			return
		}

		log.Infof("created position release orders: %+v", createdOrders)
	})

	// register position update handler: check if position is over the hard limit
	orderExecutor.TradeCollector().OnPositionUpdate(func(position *types.Position) {
		if fixedpoint.Compare(position.Base, hardLimit) > 0 {
			log.Infof("position %f is over hardlimit %f, releasing position...", position.Base.Float64(), hardLimit.Float64())
			control.EmitReleasePosition(position.Base.Sub(hardLimit), types.SideTypeSell)
		} else if fixedpoint.Compare(position.Base, hardLimit.Neg()) < 0 {
			log.Infof("position %f is over hardlimit %f, releasing position...", position.Base.Float64(), hardLimit.Float64())
			control.EmitReleasePosition(position.Base.Neg().Sub(hardLimit), types.SideTypeBuy)
		}
	})

	return control
}

// ModifiedQuantity returns sliceQuantity controlled by position risks
// For buy orders, modify sliceQuantity = min(hardLimit - position, sliceQuantity), limiting by positive position
// For sell orders, modify sliceQuantity = min(hardLimit - (-position), sliceQuantity), limiting by negative position
//
// Pass the current base position to this method, and it returns the maximum sliceQuantity for placing the orders.
// This works for both Long/Short position
func (p *PositionRiskControl) ModifiedQuantity(position fixedpoint.Value) (buyQuantity, sellQuantity fixedpoint.Value) {
	if p.sliceQuantity.IsZero() {
		buyQuantity = p.hardLimit.Sub(position)
		sellQuantity = p.hardLimit.Add(position)
		return buyQuantity, sellQuantity
	}

	buyQuantity = fixedpoint.Min(p.hardLimit.Sub(position), p.sliceQuantity)
	sellQuantity = fixedpoint.Min(p.hardLimit.Add(position), p.sliceQuantity)
	return buyQuantity, sellQuantity
}
