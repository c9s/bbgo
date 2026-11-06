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

	// PriceModeCharged values each row at the price when it was charged (MarginInterest.PriceInUSDT).
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
	// Asset is the borrowed asset (MarginInterest.BorrowedAsset)
	Asset string

	// Interest is the summed nominal interest in Asset units
	// (MarginInterest.NominalInterest), however it was paid
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

// SumRealized sums interest rows per borrowed asset and values them in USDT.
// A row paid in another asset (e.g. BNB) counts toward its RawAsset.
//
// Rows with the same Key (Asset, BorrowedAsset, IsolatedSymbol, Time) are counted once,
// because margin_interests has no unique key and two processes (e.g. `bbgo sync`
// next to `bbgo run`) can insert the same rows. The borrowed asset is in the key
// because one hour has a BNB-paid row for every borrowed asset.
//
// current: USDT = sum over paid assets of Charged[paid] x pricer(paid).
// charged: USDT = sum(row.Interest x row.PriceInUSDT), the price of the paid asset.
// A row with PriceInUSDT <= 0 is left out of USDT (its amounts still count) and
// marks the borrowed asset missing. USDT always uses price 1.
func SumRealized(rows []types.MarginInterest, mode PriceMode, pricer PriceFunc) Realized {
	mode = mode.OrDefault()
	r := Realized{
		Mode:      mode,
		Assets:    make(map[string]AssetRealized),
		TotalUSDT: fixedpoint.Zero,
	}

	seen := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		borrowed := row.BorrowedAsset()
		key := row.Key()
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}

		ar, ok := r.Assets[borrowed]
		if !ok {
			ar = AssetRealized{
				Asset:    borrowed,
				Interest: fixedpoint.Zero,
				Charged:  make(map[string]fixedpoint.Value),
				USDT:     fixedpoint.Zero,
				PriceOK:  true,
			}
		}

		ar.Interest = ar.Interest.Add(row.NominalInterest())
		charged, ok := ar.Charged[row.Asset]
		if !ok {
			charged = fixedpoint.Zero
		}
		ar.Charged[row.Asset] = charged.Add(row.Interest)
		ar.Rows++

		if mode == PriceModeCharged {
			switch {
			case row.Asset == quoteCurrency:
				ar.USDT = ar.USDT.Add(row.Interest)
			case row.PriceInUSDT.Sign() > 0:
				ar.USDT = ar.USDT.Add(row.Interest.Mul(row.PriceInUSDT))
			default:
				ar.PriceOK = false
			}
		}

		r.Assets[borrowed] = ar
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
