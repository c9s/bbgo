package margininterest

import (
	"fmt"
	"sort"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

// PriceMode selects how realized interest is valued in USDT.
type PriceMode string

const (
	// PriceModeCurrent values the summed interest at the current price.
	PriceModeCurrent PriceMode = "current"

	// PriceModeCharged values each row at the price when it was charged (MarginInterestValuation.InterestUSDT).
	PriceModeCharged PriceMode = "charged"
)

// Validate accepts "", "current" and "charged". An empty mode means charged.
func (m PriceMode) Validate() error {
	switch m {
	case "", PriceModeCurrent, PriceModeCharged:
		return nil
	}

	return fmt.Errorf("invalid realized price mode %q, expected %q or %q", m, PriceModeCurrent, PriceModeCharged)
}

// OrDefault returns PriceModeCharged for an empty mode.
func (m PriceMode) OrDefault() PriceMode {
	if m == "" {
		return PriceModeCharged
	}
	return m
}

// AssetRealized is the interest already charged on one asset in a range
type AssetRealized struct {
	Asset string

	// Interest is the summed interest in asset units
	Interest fixedpoint.Value

	USDT fixedpoint.Value
	Rows int

	// PriceOK is false if any row (charged) or the asset (current) could not be priced
	PriceOK bool
}

// Realized is the interest already charged in a range, per asset and in total
type Realized struct {
	Mode         PriceMode
	Assets       map[string]AssetRealized
	TotalUSDT    fixedpoint.Value
	MissingPrice []string
}

// SumRealized sums the valued interest rows per paid asset and values them in USDT.
//
// The rows are not deduplicated: there is one valuation per margin_interests
// row, and margin_interests stores each charge once (its tx_id is unique).
//
// current: USDT = sum(interest) x pricer(asset).
// charged: USDT = sum(row.InterestUSDT). A row without InterestUSDT is left out
// of USDT (its Interest still counts) and marks the asset missing.
// USDT always uses price 1.
func SumRealized(rows []types.MarginInterestValuation, mode PriceMode, pricer PriceFunc) Realized {
	mode = mode.OrDefault()
	r := Realized{
		Mode:      mode,
		Assets:    make(map[string]AssetRealized),
		TotalUSDT: fixedpoint.Zero,
	}

	for _, row := range rows {
		asset := row.PaidAsset
		ar, ok := r.Assets[asset]
		if !ok {
			ar = AssetRealized{Asset: asset, Interest: fixedpoint.Zero, USDT: fixedpoint.Zero, PriceOK: true}
		}

		ar.Interest = ar.Interest.Add(row.Interest)
		ar.Rows++

		if mode == PriceModeCharged {
			switch {
			case asset == quoteCurrency:
				ar.USDT = ar.USDT.Add(row.Interest)
			case row.InterestUSDT != nil:
				ar.USDT = ar.USDT.Add(*row.InterestUSDT)
			default:
				ar.PriceOK = false
			}
		}

		r.Assets[asset] = ar
	}

	for asset, ar := range r.Assets {
		if mode == PriceModeCurrent {
			if px, ok := priceOf(pricer, asset); ok {
				ar.USDT = ar.Interest.Mul(px)
			} else {
				ar.PriceOK = false
			}
			r.Assets[asset] = ar
		}

		if !ar.PriceOK {
			r.MissingPrice = append(r.MissingPrice, asset)
		}

		r.TotalUSDT = r.TotalUSDT.Add(ar.USDT)
	}

	sort.Strings(r.MissingPrice)
	return r
}
