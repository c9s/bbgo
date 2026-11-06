package margininterest

import (
	"sort"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

const quoteCurrency = "USDT"

// PriceFunc resolves the USDT price of an asset. ok is false when the price is unknown.
type PriceFunc func(asset string) (price fixedpoint.Value, ok bool)

// priceOf returns 1 for USDT and otherwise asks pricer. A nil pricer or a
// non-positive price counts as unknown.
func priceOf(pricer PriceFunc, asset string) (fixedpoint.Value, bool) {
	if asset == quoteCurrency {
		return fixedpoint.One, true
	}

	if pricer == nil {
		return fixedpoint.Zero, false
	}

	p, ok := pricer(asset)
	if !ok || p.Sign() <= 0 {
		return fixedpoint.Zero, false
	}

	return p, true
}

// ProjectInterest returns principal x hours x hourlyRate, the simple interest
// Binance charges on principal over the given number of hourly charges.
//
// Binance rounds every charge up to 8 decimals, which is not modelled here:
// the real charge can be up to 1e-8 per hour above this figure.
//
// The multiplication order matters for the non-dnum build, where Mul is a
// float64 product truncated to 8 decimals:
//   - hours x rate first is not exact (744 x 0.00000105 = 0.00078119), and the
//     error grows with the principal.
//   - principal x rate first loses up to hours x 1e-8, so it is used only when
//     principal x hours overflows (about 1.24e8 at 744 hours).
func ProjectInterest(principal, hourlyRate fixedpoint.Value, hours int) fixedpoint.Value {
	if principal.Sign() <= 0 || hourlyRate.Sign() <= 0 || hours <= 0 {
		return fixedpoint.Zero
	}

	h := fixedpoint.NewFromInt(int64(hours))
	v := principal.Mul(h).Mul(hourlyRate)
	if v.IsInf() {
		v = principal.Mul(hourlyRate).Mul(h)
	}

	return v
}

// AssetProjection is the interest of one asset projected on its principal over a number of hours
type AssetProjection struct {
	Asset      string
	Principal  fixedpoint.Value
	HourlyRate fixedpoint.Value
	Hours      int

	// RateOK is false when the asset has no hourly rate; then HourlyRate and Interest are 0
	RateOK bool

	// Interest is in asset units
	Interest fixedpoint.Value

	// USDT is Interest valued in USDT. PriceOK is false when Interest > 0 has no price;
	// a zero interest is always valued 0.
	USDT    fixedpoint.Value
	PriceOK bool
}

// Projection is the interest projected over a number of hours, per asset and in total
type Projection struct {
	Hours int

	// Assets holds every projected asset except the ones in MissingRate
	Assets map[string]AssetProjection

	// TotalUSDT sums the assets that have a price
	TotalUSDT fixedpoint.Value

	// MissingRate are the assets with principal but no hourly rate
	MissingRate []string

	// MissingPrice are the assets with interest but no price
	MissingPrice []string
}

// Complete reports whether every asset with principal had both a rate and a price.
func (p Projection) Complete() bool {
	return len(p.MissingRate) == 0 && len(p.MissingPrice) == 0
}

// ProjectAssets projects the interest of each asset over the given hours, on
// the principal debts[asset].Borrowed. An asset without debt is listed with zero
// interest, so every asset is reported whether it is borrowed or not.
// Accrued interest is not principal, so only Borrowed counts.
func ProjectAssets(
	assets []string, debts types.BalanceMap, rates types.MarginNextHourlyInterestRateMap, pricer PriceFunc, hours int,
) Projection {
	p := Projection{
		Hours:     hours,
		Assets:    make(map[string]AssetProjection, len(assets)),
		TotalUSDT: fixedpoint.Zero,
	}

	for _, asset := range assets {
		if _, ok := p.Assets[asset]; ok {
			continue
		}

		principal := fixedpoint.Zero
		if b, ok := debts[asset]; ok && b.Borrowed.Sign() > 0 {
			principal = b.Borrowed
		}

		ap := AssetProjection{
			Asset:      asset,
			Principal:  principal,
			HourlyRate: fixedpoint.Zero,
			Hours:      hours,
			Interest:   fixedpoint.Zero,
			USDT:       fixedpoint.Zero,
		}

		if rate, ok := rates[asset]; ok && rate != nil && rate.HourlyRate.Sign() > 0 {
			ap.RateOK = true
			ap.HourlyRate = rate.HourlyRate
			ap.Interest = ProjectInterest(principal, rate.HourlyRate, hours)
		} else if principal.Sign() > 0 {
			p.MissingRate = append(p.MissingRate, asset)
			continue
		}

		if ap.Interest.IsZero() {
			ap.PriceOK = true
		} else if px, ok := priceOf(pricer, asset); ok {
			ap.USDT = ap.Interest.Mul(px)
			ap.PriceOK = true
			p.TotalUSDT = p.TotalUSDT.Add(ap.USDT)
		} else {
			p.MissingPrice = append(p.MissingPrice, asset)
		}

		p.Assets[asset] = ap
	}

	sort.Strings(p.MissingRate)
	sort.Strings(p.MissingPrice)
	return p
}
