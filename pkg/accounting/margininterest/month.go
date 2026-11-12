package margininterest

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
)

// AssetMonth is the interest of one asset in the UTC month: what was charged so
// far plus what the current principal will be charged until the month end.
// It is the input of a per-asset budget, in asset units or in USDT.
type AssetMonth struct {
	Asset string

	// Projected, Realized and Total are in asset units
	Projected fixedpoint.Value
	Realized  fixedpoint.Value
	Total     fixedpoint.Value

	// ProjectedUSDT, RealizedUSDT and TotalUSDT leave out the parts that have no price
	ProjectedUSDT fixedpoint.Value
	RealizedUSDT  fixedpoint.Value
	TotalUSDT     fixedpoint.Value

	// MissingRate: the asset has principal but no hourly rate, so Projected is 0
	MissingRate bool

	// MissingPrice: a projected or realized part could not be valued in USDT
	MissingPrice bool

	// MissingHistory: the realized rows of the asset may be incomplete;
	// set by the caller, which knows the sync state
	MissingHistory bool
}

// NewAssetMonth returns an AssetMonth with zero interest.
func NewAssetMonth(asset string) AssetMonth {
	return AssetMonth{
		Asset:         asset,
		Projected:     fixedpoint.Zero,
		Realized:      fixedpoint.Zero,
		Total:         fixedpoint.Zero,
		ProjectedUSDT: fixedpoint.Zero,
		RealizedUSDT:  fixedpoint.Zero,
		TotalUSDT:     fixedpoint.Zero,
	}
}

// AmountComplete reports whether Total (asset units) is the full month figure.
func (a AssetMonth) AmountComplete() bool {
	return !a.MissingRate && !a.MissingHistory
}

// Complete reports whether both Total and TotalUSDT are the full month figures.
func (a AssetMonth) Complete() bool {
	return a.AmountComplete() && !a.MissingPrice
}

// MonthByAsset combines the projection and the realized interest per asset.
// Every projected asset (borrowed or not, with or without a rate) and every
// asset with realized rows is listed.
func MonthByAsset(p Projection, r Realized) map[string]AssetMonth {
	out := make(map[string]AssetMonth, len(p.Assets)+len(p.MissingRate)+len(r.Assets))

	get := func(asset string) AssetMonth {
		if m, ok := out[asset]; ok {
			return m
		}
		return NewAssetMonth(asset)
	}

	for asset, ap := range p.Assets {
		m := get(asset)
		m.Projected = ap.Interest
		if ap.PriceOK {
			m.ProjectedUSDT = ap.USDT
		} else {
			m.MissingPrice = true
		}
		out[asset] = m
	}

	for _, asset := range p.MissingRate {
		m := get(asset)
		m.MissingRate = true
		out[asset] = m
	}

	for asset, ar := range r.Assets {
		m := get(asset)
		m.Realized = ar.Interest
		m.RealizedUSDT = ar.USDT
		if !ar.PriceOK {
			m.MissingPrice = true
		}
		out[asset] = m
	}

	for asset, m := range out {
		m.Total = m.Projected.Add(m.Realized)
		m.TotalUSDT = m.ProjectedUSDT.Add(m.RealizedUSDT)
		out[asset] = m
	}

	return out
}
