package xnav

import (
	"time"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/pricesolver"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/types/asset"
)

func NewAssetMapFromBalanceMap(
	priceSolver *pricesolver.SimplePriceSolver, priceTime time.Time, m types.BalanceMap, fiat string,
) asset.Map {
	assets := make(asset.Map)

	btcInUSD, hasBtcPrice := priceSolver.ResolvePrice("BTC", fiat, "USDT")
	if !hasBtcPrice {
		logrus.Warnf("AssetMap: unable to resolve price for BTC")
	}

	for cu, b := range m {
		total := b.Total()
		netAsset := b.Net()
		debt := b.Debt()

		if total.IsZero() && netAsset.IsZero() && debt.IsZero() {
			continue
		}

		asset := asset.Asset{
			Currency:  cu,
			Total:     total,
			Time:      priceTime,
			Locked:    b.Locked,
			Available: b.Available,
			Borrowed:  b.Borrowed,
			Interest:  b.Interest,
			NetAsset:  netAsset,
		}

		if assetPrice, ok := priceSolver.ResolvePrice(cu, fiat, "USDT"); ok {
			asset.PriceInUSD = assetPrice
			asset.NetAssetInUSD = netAsset.Mul(assetPrice)
			if hasBtcPrice {
				asset.NetAssetInBTC = asset.NetAssetInUSD.Div(btcInUSD)
			}

			asset.DebtInUSD = debt.Mul(assetPrice)
			asset.InterestInUSD = b.Interest.Mul(assetPrice)
		} else {
			logrus.Warnf("AssetMap: unable to resolve price for %s", cu)
		}

		assets[cu] = asset
	}

	return assets
}
