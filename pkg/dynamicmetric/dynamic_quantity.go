package dynamicmetric

import (
	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/pkg/errors"
)

// DynamicQuantitySet uses multiple dynamic quantity rules to calculate the total quantity
type DynamicQuantitySet []DynamicQuantity

// Initialize dynamic quantity set
func (d *DynamicQuantitySet) Initialize(symbol string, session *bbgo.ExchangeSession) {
	for i := range *d {
		(*d)[i].Initialize(symbol, session)
	}
}

// GetQuantity returns the quantity
func (d *DynamicQuantitySet) GetQuantity() (fixedpoint.Value, error) {
	quantity := fixedpoint.Zero
	for i := range *d {
		v, err := (*d)[i].getQuantity()
		if err != nil {
			return fixedpoint.Zero, err
		}
		quantity = quantity.Add(v)
	}

	return quantity, nil
}

type DynamicQuantity struct {
	// LinRegQty calculates quantity based on LinReg slope
	LinRegDynamicQuantity *DynamicQuantityLinReg `json:"linRegDynamicQuantity"`
}

// Initialize dynamic quantity
func (d *DynamicQuantity) Initialize(symbol string, session *bbgo.ExchangeSession) {
	switch {
	case d.LinRegDynamicQuantity != nil:
		d.LinRegDynamicQuantity.initialize(symbol, session)
	}
}

func (d *DynamicQuantity) IsEnabled() bool {
	return d.LinRegDynamicQuantity != nil
}

// getQuantity returns quantity
func (d *DynamicQuantity) getQuantity() (fixedpoint.Value, error) {
	switch {
	case d.LinRegDynamicQuantity != nil:
		return d.LinRegDynamicQuantity.getQuantity()
	default:
		return fixedpoint.Zero, errors.New("dynamic quantity is not enabled")
	}
}

// DynamicQuantityLinReg uses LinReg slope to calculate quantity
type DynamicQuantityLinReg struct {
	// DynamicQuantityLinRegScale is used to define the quantity range with the given parameters.
	DynamicQuantityLinRegScale *bbgo.PercentageScale `json:"dynamicQuantityLinRegScale"`

	// QuantityLinReg to define the interval and window of the LinReg
	QuantityLinReg *indicator.LinReg `json:"quantityLinReg"`
}

// initialize LinReg dynamic quantity
func (d *DynamicQuantityLinReg) initialize(symbol string, session *bbgo.ExchangeSession) {
	// Subscribe for LinReg
	session.Subscribe(types.KLineChannel, symbol, types.SubscribeOptions{
		Interval: d.QuantityLinReg.Interval,
	})

	// Initialize LinReg
	kLineStore, _ := session.MarketDataStore(symbol)
	d.QuantityLinReg.BindK(session.MarketDataStream, symbol, d.QuantityLinReg.Interval)
	if klines, ok := kLineStore.KLinesOfInterval(d.QuantityLinReg.Interval); ok {
		d.QuantityLinReg.LoadK((*klines)[0:])
	}
}

// getQuantity returns quantity
func (d *DynamicQuantityLinReg) getQuantity() (fixedpoint.Value, error) {
	v, err := d.DynamicQuantityLinRegScale.Scale(d.QuantityLinReg.LastRatio())
	if err != nil {
		return fixedpoint.Zero, err
	}
	return fixedpoint.NewFromFloat(v), nil
}
