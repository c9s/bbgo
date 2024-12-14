package asset

import (
	"time"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/pricesolver"
	"github.com/c9s/bbgo/pkg/types"
)

func NewMapFromBalanceMap(
	priceSolver *pricesolver.SimplePriceSolver, priceTime time.Time, m types.BalanceMap, fiat string,
) types.AssetMap {
	assets := make(types.AssetMap)

	btcInUSD, hasBtcPrice := priceSolver.ResolvePrice("BTC", fiat, "USDT")
	if !hasBtcPrice {
		logrus.Warnf("AssetMap: unable to resolve price for BTC")
	}

	for currency, b := range m {

		total := b.Total()
		netAsset := b.Net()
		debt := b.Debt()

		if total.IsZero() && netAsset.IsZero() && debt.IsZero() {
			continue
		}

		asset := types.Asset{
			Currency:  currency,
			Total:     total,
			Time:      priceTime,
			Locked:    b.Locked,
			Available: b.Available,
			Borrowed:  b.Borrowed,
			Interest:  b.Interest,
			NetAsset:  netAsset,
		}

		if assetPrice, ok := priceSolver.ResolvePrice(currency, fiat, "USDT"); ok {
			asset.PriceInUSD = assetPrice
			asset.InUSD = netAsset.Mul(assetPrice)
			if hasBtcPrice {
				asset.InBTC = asset.InUSD.Div(btcInUSD)
			}
		} else {
			logrus.Warnf("AssetMap: unable to resolve price for %s", currency)
		}

		assets[currency] = asset
	}

	return assets
}
