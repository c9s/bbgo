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

type interestRowKey struct {
	asset, isolatedSymbol string
	timeMs                int64
}

// SumRealized sums interest rows per asset and values them in USDT.
//
// Rows with the same (Asset, IsolatedSymbol, Time) are counted once, because
// margin_interests has no unique key and two processes (e.g. `bbgo sync`
// next to `bbgo run`) can insert the same rows.
//
// current: USDT = sum(interest) x price(asset).
// charged: USDT = sum(interest x row.PriceInUSDT). A row with PriceInUSDT <= 0 is
// left out of USDT (its Interest still counts) and marks the asset missing.
// USDT always uses price 1.
func SumRealized(rows []types.MarginInterest, mode PriceMode, price PriceFunc) Realized {
	mode = mode.OrDefault()
	r := Realized{
		Mode:      mode,
		Assets:    make(map[string]AssetRealized),
		TotalUSDT: fixedpoint.Zero,
	}

	seen := make(map[interestRowKey]struct{}, len(rows))
	for _, row := range rows {
		key := interestRowKey{row.Asset, row.IsolatedSymbol, row.Time.Time().UnixMilli()}
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}

		ar, ok := r.Assets[row.Asset]
		if !ok {
			ar = AssetRealized{Asset: row.Asset, Interest: fixedpoint.Zero, USDT: fixedpoint.Zero, PriceOK: true}
		}

		ar.Interest = ar.Interest.Add(row.Interest)
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

		r.Assets[row.Asset] = ar
	}

	for asset, ar := range r.Assets {
		if mode == PriceModeCurrent {
			if px, ok := priceOf(price, asset); ok {
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
