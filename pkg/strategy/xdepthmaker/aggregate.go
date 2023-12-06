package xdepthmaker

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func aggregatePrice(pvs types.PriceVolumeSlice, requiredQuantity fixedpoint.Value) (price fixedpoint.Value) {
	q := requiredQuantity
	totalAmount := fixedpoint.Zero

	if len(pvs) == 0 {
		price = fixedpoint.Zero
		return price
	} else if pvs[0].Volume.Compare(requiredQuantity) >= 0 {
		return pvs[0].Price
	}

	for i := 0; i < len(pvs); i++ {
		pv := pvs[i]
		if pv.Volume.Compare(q) >= 0 {
			totalAmount = totalAmount.Add(q.Mul(pv.Price))
			break
		}

		q = q.Sub(pv.Volume)
		totalAmount = totalAmount.Add(pv.Volume.Mul(pv.Price))
	}

	price = totalAmount.Div(requiredQuantity.Sub(q))
	return price
}
