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

// AssetRealized is the interest already charged on one borrowed asset in a range
type AssetRealized struct {
	// Asset is the borrowed asset (MarginInterestValuation.BorrowedAsset)
	Asset string

	// Interest is the summed interest in Asset units, however it was paid: a row
	// paid in another asset (e.g. BNB) counts at its BorrowedInterest, the
	// nominal principal x rate
	Interest fixedpoint.Value

	// Charged is what was actually charged, per paid asset: Asset itself, or
	// e.g. BNB for interest paid with BNB deduction
	Charged map[string]fixedpoint.Value

	// USDT values Charged, so a discount of paying in another asset is included
	USDT fixedpoint.Value
	Rows int

	// PriceOK is false if any row (charged) or any paid asset (current) could not be priced
	PriceOK bool
}

// Realized is the interest already charged in a range, per borrowed asset and in total
type Realized struct {
	Mode         PriceMode
	Assets       map[string]AssetRealized
	TotalUSDT    fixedpoint.Value
	MissingPrice []string
}

// SumRealized sums the valued interest rows per borrowed asset and values them
// in USDT. A row paid in another asset (e.g. BNB) counts toward its
// BorrowedAsset. A row whose borrowed asset is unknown (BorrowedAsset empty or no
// BorrowedInterest) cannot be attributed and is left out: the caller filters
// and reports those.
//
// The rows are not deduplicated: there is one valuation per margin_interests
// row, and margin_interests stores each charge once (its tx_id is unique).
//
// current: USDT = sum over paid assets of Charged[paid] x pricer(paid).
// charged: USDT = sum(row.InterestUSDT), the value of the paid asset when it
// was charged. A row without InterestUSDT is left out of USDT (its amounts
// still count) and marks the borrowed asset missing. USDT always uses price 1.
func SumRealized(rows []types.MarginInterestValuation, mode PriceMode, pricer PriceFunc) Realized {
	mode = mode.OrDefault()
	r := Realized{
		Mode:      mode,
		Assets:    make(map[string]AssetRealized),
		TotalUSDT: fixedpoint.Zero,
	}

	for _, row := range rows {
		asset := row.BorrowedAsset
		if asset == "" || row.BorrowedInterest == nil {
			continue
		}

		ar, ok := r.Assets[asset]
		if !ok {
			ar = AssetRealized{
				Asset:    asset,
				Interest: fixedpoint.Zero,
				Charged:  make(map[string]fixedpoint.Value),
				USDT:     fixedpoint.Zero,
				PriceOK:  true,
			}
		}

		ar.Interest = ar.Interest.Add(*row.BorrowedInterest)
		charged, ok := ar.Charged[row.PaidAsset]
		if !ok {
			charged = fixedpoint.Zero
		}
		ar.Charged[row.PaidAsset] = charged.Add(row.Interest)
		ar.Rows++

		if mode == PriceModeCharged {
			switch {
			case row.PaidAsset == quoteCurrency:
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
			for paid, amount := range ar.Charged {
				if px, ok := priceOf(pricer, paid); ok {
					ar.USDT = ar.USDT.Add(amount.Mul(px))
				} else {
					ar.PriceOK = false
				}
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
